// Sandbox network policy stored on a tenant sandbox config.
//
// One sandbox config carries one network policy, applied to every sandbox
// created from it — chat sessions, skill installs and connectivity probes
// alike. The provider-facing translation lives in internal/sandbox; this file
// is only the stored shape, its secret handling, and its validation.

package types

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// SandboxNetworkPolicy is the admin-facing network policy for one sandbox
// config. DenyEgressByDefault is phrased so the zero value IS the intended
// default (outbound open). Inbound is not a switch: it is always
// credential-required. AllowPublicInbound stays on the wire so old payloads
// still decode, then is cleared on save and ignored at resolve.
type SandboxNetworkPolicy struct {
	// DenyEgressByDefault installs a 0.0.0.0/0 deny-all, after which only
	// AllowOut (and L7 rule targets) can reach the network. false allows
	// public egress, which is what skill installs need.
	DenyEgressByDefault bool `json:"deny_egress_by_default,omitempty"`

	// AllowPublicInbound is accepted on the wire for old payloads and then
	// cleared. Inbound is always credential-required; a true value has no
	// runtime effect.
	AllowPublicInbound bool `json:"allow_public_inbound,omitempty"`

	// AllowOut accepts IPv4, IPv4 CIDR, a DNS name, or a single-label
	// wildcard such as "*.example.com". Domain entries are only meaningful
	// together with a deny-all; see ValidateSandboxNetworkPolicy.
	AllowOut []string `json:"allow_out,omitempty"`

	// DenyOut accepts IPv4 and IPv4 CIDR only. Neither provider can deny a
	// domain: denial is a pure longest-prefix match on the destination IP.
	DenyOut []string `json:"deny_out,omitempty"`

	// CubeRules are validated whenever present and consumed only by the Cube
	// provider adapter.
	CubeRules []CubeEgressRule `json:"cube_rules,omitempty"`

	// E2BHostRules are validated whenever present and consumed only by the E2B
	// provider adapter.
	E2BHostRules []E2BHostRule `json:"e2b_host_rules,omitempty"`
}

// CubeEgressRule mirrors cubesandbox.Rule. Match fields are AND-ed; Methods is
// OR-ed internally.
type CubeEgressRule struct {
	Name    string   `json:"name"`
	Scheme  string   `json:"scheme,omitempty"`
	SNI     string   `json:"sni,omitempty"`
	Host    string   `json:"host,omitempty"`
	Methods []string `json:"methods,omitempty"`
	Path    string   `json:"path,omitempty"`

	// Deny inverts the action, which defaults to allow. A deny rule still
	// needs Host or SNI: the target has to reach CubeEgress for it to answer
	// with a request-level 403 instead of the network layer dropping it.
	Deny bool `json:"deny,omitempty"`

	// Audit is none | metadata | full. Empty uses the server default.
	Audit string `json:"audit,omitempty"`

	// Inject adds credential headers on allowed HTTPS requests.
	Inject []CubeHeaderInject `json:"inject,omitempty"`
}

// CubeHeaderInject mirrors cubesandbox.Inject. Secret is a credential and is
// encrypted at rest; Header and Format are not.
type CubeHeaderInject struct {
	Header string `json:"header"`
	Secret string `json:"secret"`
	Format string `json:"format,omitempty"`
}

// E2BHostRule mirrors one entry of e2b.NetworkConfig.Rules. Host must also
// appear in AllowOut: a rule grants no egress on its own.
type E2BHostRule struct {
	Host string `json:"host"`
	// Headers values are credentials and are encrypted at rest; names stay
	// readable so operators can see which headers are injected.
	Headers map[string]string `json:"headers,omitempty"`
}

// CloneWithSecrets deep-copies p, passing every credential-bearing field
// through transform. Both the encrypt (Value), decrypt (Scan) and mask
// (SandboxConfigForResponse) directions share it so they cannot drift about
// which fields count as secrets. The receiver is never mutated.
func (p *SandboxNetworkPolicy) CloneWithSecrets(
	transform func(string) string,
) *SandboxNetworkPolicy {
	if p == nil {
		return nil
	}
	out := *p
	out.AllowOut = append([]string(nil), p.AllowOut...)
	out.DenyOut = append([]string(nil), p.DenyOut...)

	if p.CubeRules != nil {
		rules := make([]CubeEgressRule, len(p.CubeRules))
		for i, rule := range p.CubeRules {
			copied := rule
			copied.Methods = append([]string(nil), rule.Methods...)
			if rule.Inject != nil {
				injects := make([]CubeHeaderInject, len(rule.Inject))
				for j, inject := range rule.Inject {
					injects[j] = inject
					injects[j].Secret = transform(inject.Secret)
				}
				copied.Inject = injects
			}
			rules[i] = copied
		}
		out.CubeRules = rules
	}

	if p.E2BHostRules != nil {
		hostRules := make([]E2BHostRule, len(p.E2BHostRules))
		for i, rule := range p.E2BHostRules {
			copied := rule
			if rule.Headers != nil {
				headers := make(map[string]string, len(rule.Headers))
				for name, value := range rule.Headers {
					headers[name] = transform(value)
				}
				copied.Headers = headers
			}
			hostRules[i] = copied
		}
		out.E2BHostRules = hostRules
	}
	return &out
}

// e2bNetworkLimits mirror the hard limits E2B enforces server-side. Checking
// them here turns a 400 at sandbox-create time into a message on the form the
// admin is looking at.
const (
	e2bMaxRuleDomains      = 10
	e2bMaxHeadersPerRule   = 20
	e2bMaxRuleDomainLength = 128
	e2bMaxHeaderNameLength = 64
	e2bMaxHeaderValueLen   = 2048
)

// DenyAllIPv4 is the deny-all entry both providers expect in a deny list. E2B
// calls it ALL_TRAFFIC in its error messages but accepts this CIDR (its own
// SDK exports it as e2b.AllTraffic with this value).
const DenyAllIPv4 = "0.0.0.0/0"

var (
	cubeAuditLevels = map[string]bool{"": true, "none": true, "metadata": true, "full": true}
	httpMethods     = map[string]bool{
		"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true,
		"DELETE": true, "CONNECT": true, "OPTIONS": true, "TRACE": true,
	}
)

// ValidateSandboxNetworkPolicy rejects a policy the provider would refuse, or
// would silently accept without doing what the admin meant. Errors are plain
// values; the service layer wraps them as bad-request responses.
func ValidateSandboxNetworkPolicy(cfg *TenantSandboxConfig) error {
	if cfg == nil || cfg.Network == nil {
		return nil
	}
	p := cfg.Network
	backend := strings.TrimSpace(strings.ToLower(cfg.SandboxType))

	if backend == "docker" &&
		(len(p.AllowOut) > 0 || len(p.DenyOut) > 0 ||
			len(p.CubeRules) > 0 || len(p.E2BHostRules) > 0) {
		return errors.New(
			"the docker backend can only switch egress on or off as a whole, not by IP, domain or HTTP rule. " +
				"Clear the allow/deny lists, or switch to the cube / e2b backend")
	}

	allowKeys := make(map[string]string, len(p.AllowOut))
	hasDomainAllow := false
	for _, target := range p.AllowOut {
		kind, key, err := classifyNetworkTarget(target, true)
		if err != nil {
			return fmt.Errorf("allow_out %q: %w", target, err)
		}
		if prior, seen := allowKeys[key]; seen {
			return fmt.Errorf("allow_out entries %q and %q are the same target; keep only one", prior, target)
		}
		allowKeys[key] = target
		if kind == targetDomain {
			hasDomainAllow = true
		}
	}

	denyKeys := make(map[string]string, len(p.DenyOut))
	deniesEverything := p.DenyEgressByDefault
	for _, target := range p.DenyOut {
		kind, key, err := classifyNetworkTarget(target, false)
		if err != nil {
			return fmt.Errorf("deny_out %q: %w", target, err)
		}
		if kind == targetDomain {
			return fmt.Errorf(
				"deny_out does not support domains (%q): denial matches destination IPs only. "+
					"To allow only a few domains, turn on \"Deny by default\" and list them in allow_out", target)
		}
		if prior, seen := denyKeys[key]; seen {
			return fmt.Errorf("deny_out entries %q and %q are the same target; keep only one", prior, target)
		}
		denyKeys[key] = target
		if key == DenyAllIPv4 {
			deniesEverything = true
		}
	}

	if hasDomainAllow && !deniesEverything {
		return errors.New(
			"when allow_out contains a domain, all other traffic must be denied as a fallback: " +
				"turn on \"Deny by default\" or add 0.0.0.0/0 to deny_out. " +
				"Otherwise IPs not learned through DNS are still allowed and the allowlist does nothing")
	}

	if err := validateCubeEgressRules(p.CubeRules); err != nil {
		return err
	}
	return validateE2BHostRules(p.E2BHostRules, allowKeys)
}

// DenyOutCoversAllIPv4 reports whether a deny list blocks every IPv4
// destination. That is the second of the two ways an admin can express the
// deny-all fallback — the first being DenyEgressByDefault — and
// ValidateSandboxNetworkPolicy treats them as equivalent, so every consumer
// that asks "does this policy deny by default?" has to accept both or it will
// misjudge a config the admin was allowed to save.
//
// It normalises through classifyNetworkTarget rather than comparing strings so
// it cannot disagree with the validator about which entries count: any /0
// collapses onto 0.0.0.0/0, and surrounding whitespace is ignored. Entries the
// validator would reject are skipped here; rejecting them is its job.
func DenyOutCoversAllIPv4(denyOut []string) bool {
	for _, target := range denyOut {
		kind, key, err := classifyNetworkTarget(target, false)
		if err != nil || kind != targetIP {
			continue
		}
		if key == DenyAllIPv4 {
			return true
		}
	}
	return false
}

// CanonicalizeDenyOut rewrites any IPv4 /0 onto DenyAllIPv4 so providers that
// string-match 0.0.0.0/0 (E2B's ALL_TRAFFIC) see the same deny-all the
// validator accepted. Other entries are copied as typed. Nil input stays nil.
func CanonicalizeDenyOut(denyOut []string) []string {
	if denyOut == nil {
		return nil
	}
	out := make([]string, 0, len(denyOut))
	for _, target := range denyOut {
		kind, key, err := classifyNetworkTarget(target, false)
		if err == nil && kind == targetIP && key == DenyAllIPv4 {
			out = append(out, DenyAllIPv4)
			continue
		}
		out = append(out, target)
	}
	return out
}

type networkTargetKind int

const (
	targetIP networkTargetKind = iota
	targetDomain
)

// classifyNetworkTarget normalises one allow/deny entry and reports whether it
// is an address or a domain. key is the normalised form both providers dedupe
// on: a bare IPv4 collapses onto its /32, a domain is lowercased with the
// trailing dot removed.
func classifyNetworkTarget(
	target string,
	allowWildcard bool,
) (kind networkTargetKind, key string, err error) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return 0, "", errors.New("must not be empty")
	}
	if strings.ContainsAny(trimmed, " \t") {
		return 0, "", errors.New("must not contain spaces")
	}
	if ip := net.ParseIP(trimmed); ip != nil {
		if ip.To4() == nil {
			return 0, "", errors.New("IPv6 is not supported yet")
		}
		return targetIP, ip.String() + "/32", nil
	}
	if _, network, cidrErr := net.ParseCIDR(trimmed); cidrErr == nil {
		if network.IP.To4() == nil {
			return 0, "", errors.New("IPv6 CIDR is not supported yet")
		}
		return targetIP, network.String(), nil
	}
	return classifyDomainTarget(trimmed, allowWildcard)
}

func classifyDomainTarget(
	trimmed string,
	allowWildcard bool,
) (networkTargetKind, string, error) {
	domain := strings.ToLower(strings.TrimSuffix(trimmed, "."))
	if strings.Contains(domain, ":") {
		return 0, "", errors.New("must not include a port")
	}
	wildcard := false
	if strings.HasPrefix(domain, "*.") {
		if !allowWildcard {
			return 0, "", errors.New("wildcard domains are not supported here")
		}
		wildcard = true
		domain = strings.TrimPrefix(domain, "*.")
	}
	if strings.Trim(domain, "0123456789.") == "" {
		return 0, "", errors.New("not a valid IPv4 address")
	}
	if domain == "" || strings.Contains(domain, "*") {
		return 0, "", errors.New("a wildcard can only be a single-label prefix, such as *.example.com")
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") ||
		strings.Contains(domain, "..") {
		return 0, "", errors.New("not a valid domain")
	}
	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return 0, "", errors.New("not a valid domain")
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return 0, "", errors.New("not a valid domain")
		}
		for _, r := range label {
			isAlnum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
			if !isAlnum && r != '-' {
				return 0, "", errors.New("not a valid domain")
			}
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return 0, "", errors.New("not a valid domain")
		}
	}
	key := domain
	if wildcard {
		key = "*." + domain
	}
	return targetDomain, key, nil
}

func validateCubeEgressRules(rules []CubeEgressRule) error {
	names := make(map[string]bool, len(rules))
	for _, rule := range rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			return errors.New("every Cube HTTP rule needs a name, used for auditing and template merging")
		}
		if names[name] {
			return fmt.Errorf("cube HTTP rule name %q is duplicated", name)
		}
		names[name] = true

		if strings.TrimSpace(rule.Host) == "" && strings.TrimSpace(rule.SNI) == "" {
			return fmt.Errorf(
				"cube HTTP rule %q must set host or sni: the network layer reads allowed targets from these only, "+
					"so a rule with only method / path never reaches CubeEgress", name)
		}
		if host := strings.TrimSpace(rule.Host); host != "" {
			if _, _, err := classifyNetworkTarget(hostWithoutPort(host), true); err != nil {
				return fmt.Errorf("cube HTTP rule %q host %q: %w", name, rule.Host, err)
			}
		}
		if sni := strings.TrimSpace(rule.SNI); sni != "" {
			kind, _, err := classifyNetworkTarget(sni, true)
			if err != nil {
				return fmt.Errorf("cube HTTP rule %q sni %q: %w", name, rule.SNI, err)
			}
			if kind != targetDomain {
				return fmt.Errorf("cube HTTP rule %q sni must be a domain, not an IP", name)
			}
		}
		switch strings.ToLower(strings.TrimSpace(rule.Scheme)) {
		case "", "http", "https":
		default:
			return fmt.Errorf("cube HTTP rule %q scheme must be http or https", name)
		}
		if !cubeAuditLevels[strings.ToLower(strings.TrimSpace(rule.Audit))] {
			return fmt.Errorf("cube HTTP rule %q audit must be none, metadata or full", name)
		}
		for _, method := range rule.Methods {
			if !httpMethods[strings.ToUpper(strings.TrimSpace(method))] {
				return fmt.Errorf("cube HTTP rule %q method %q is not a standard HTTP method", name, method)
			}
		}
		injectHeaders := make(map[string]bool, len(rule.Inject))
		for _, inject := range rule.Inject {
			header := strings.TrimSpace(inject.Header)
			if header == "" {
				return fmt.Errorf("cube HTTP rule %q injected header name must not be empty", name)
			}
			if err := validateHTTPHeaderName(header); err != nil {
				return fmt.Errorf("rule %q injected header name %q: %w", name, header, err)
			}
			if injectHeaders[header] {
				return fmt.Errorf("cube HTTP rule %q injected header name %q is duplicated", name, header)
			}
			injectHeaders[header] = true
			if err := validateHTTPHeaderValue(inject.Secret); err != nil {
				return fmt.Errorf("rule %q injected header %q value: %w", name, header, err)
			}
			if err := validateHTTPHeaderValue(inject.Format); err != nil {
				return fmt.Errorf("rule %q injected header %q format: %w", name, header, err)
			}
			if rule.Deny && inject.Secret != "" {
				return fmt.Errorf("cube HTTP rule %q is a deny rule; injected headers have no effect", name)
			}
		}
	}
	return nil
}

func hostWithoutPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func validateE2BHostRules(rules []E2BHostRule, allowKeys map[string]string) error {
	if len(rules) > e2bMaxRuleDomains {
		return fmt.Errorf("e2b allows at most %d host rule domains per sandbox", e2bMaxRuleDomains)
	}
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		host := strings.TrimSpace(rule.Host)
		if host == "" {
			return errors.New("e2b host rule host must not be empty")
		}
		if len(host) > e2bMaxRuleDomainLength {
			return fmt.Errorf("e2b host rule host %q exceeds %d characters", host, e2bMaxRuleDomainLength)
		}
		kind, key, err := classifyNetworkTarget(host, true)
		if err != nil {
			return fmt.Errorf("e2b host rule host %q: %w", host, err)
		}
		if kind != targetDomain {
			return fmt.Errorf("e2b host rule host %q must be a domain", host)
		}
		if seen[key] {
			return fmt.Errorf("e2b host rule %q is duplicated: each domain can have only one rule", host)
		}
		seen[key] = true

		if !allowListCovers(allowKeys, key) {
			return fmt.Errorf(
				"e2b host rule host %q must also appear in allow_out: "+
					"the rule only injects headers and does not grant egress", host)
		}
		if len(rule.Headers) > e2bMaxHeadersPerRule {
			return fmt.Errorf("e2b host rule %q allows at most %d headers", host, e2bMaxHeadersPerRule)
		}
		for name, value := range rule.Headers {
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("e2b host rule %q header name must not be empty", host)
			}
			if err := validateHTTPHeaderName(name); err != nil {
				return fmt.Errorf("host rule %q header name %q: %w", host, name, err)
			}
			if len(name) > e2bMaxHeaderNameLength {
				return fmt.Errorf("e2b host rule %q header name exceeds %d characters",
					host, e2bMaxHeaderNameLength)
			}
			if err := validateHTTPHeaderValue(value); err != nil {
				return fmt.Errorf("host rule %q header %q value: %w", host, name, err)
			}
			if len(value) > e2bMaxHeaderValueLen {
				return fmt.Errorf("e2b host rule %q header value exceeds %d characters",
					host, e2bMaxHeaderValueLen)
			}
		}
	}
	return nil
}

// validateHTTPHeaderName rejects names that cannot be a single HTTP field
// name. CR/LF would let an injected header smuggle a second field through
// CubeEgress / E2B request transforms.
func validateHTTPHeaderName(name string) error {
	if name == "" {
		return errors.New("must not be empty")
	}
	for _, r := range name {
		if !isHTTPTokenChar(r) {
			return errors.New("not a valid HTTP header name")
		}
	}
	return nil
}

// validateHTTPHeaderValue rejects CR, LF and NUL, which would terminate the
// field and let the rest of the string become a smuggled header or request.
// Empty is allowed (unset / placeholder).
func validateHTTPHeaderValue(value string) error {
	if strings.ContainsAny(value, "\r\n\x00") {
		return errors.New("must not contain line breaks or NUL")
	}
	return nil
}

func isHTTPTokenChar(r rune) bool {
	if r <= 32 || r >= 127 {
		return false
	}
	switch r {
	case '(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}':
		return false
	}
	return true
}

// allowListCovers reports whether an allow entry authorises host: either the
// exact name, or a wildcard whose suffix matches. Mirrors both providers'
// rule that "*.example.com" covers subdomains but not the apex.
func allowListCovers(allowKeys map[string]string, host string) bool {
	if _, ok := allowKeys[host]; ok {
		return true
	}
	for key := range allowKeys {
		suffix, ok := strings.CutPrefix(key, "*.")
		if !ok {
			continue
		}
		if strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

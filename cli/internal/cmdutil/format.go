package cmdutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jrmatherly/ai-kb/cli/internal/config"
	"github.com/jrmatherly/ai-kb/cli/internal/format"
	"github.com/jrmatherly/ai-kb/cli/internal/output"
)

// FormatMode is the resolved --format value (typed enum).
type FormatMode string

const (
	FormatText   FormatMode = "text"
	FormatJSON   FormatMode = "json"
	FormatNDJSON FormatMode = "ndjson"
)

// DefaultFormatMode is the mode used when none of --format, AIKB_FORMAT and
// config.yaml defaults.format is set.
const DefaultFormatMode = FormatJSON

// FormatOptions captures the resolved --format + --jq state for a command.
// Mode is one of FormatText / FormatJSON / FormatNDJSON, or "" before
// ResolveDefault has been called.
type FormatOptions struct {
	Mode FormatMode
	JQ   string
	TTY  bool // ResolveDefault populates; Emit reads for indent decision
}

// AddFormatFlag attaches the --jq projection field hints (in cmd.Long) for
// commands that honor --format. The flags themselves are registered as
// persistent globals at the root, so this helper no longer re-registers
// them; it only appends documentation. Callers that don't need field
// hints can skip this call entirely.
func AddFormatFlag(cmd *cobra.Command, fieldHints ...string) {
	if len(fieldHints) > 0 {
		sorted := append([]string(nil), fieldHints...)
		sort.Strings(sorted)
		// Fields live under .data in the {ok,data,meta} envelope, so --jq must
		// be rooted there: `--jq '.data.<field>'` (object) or
		// `--jq '.data[].<field>'` (list). A bare `--jq '.<field>'` matches the
		// envelope top level and silently returns null, so spell out the path so
		// agents don't ship broken projections.
		hdr := "\n\nJSON fields available under .data (project with --jq '.data.<field>', or '.data[].<field>' for lists):\n  " +
			strings.Join(sorted, "\n  ")
		if cmd.Long != "" {
			cmd.Long += hdr
		} else {
			cmd.Long = strings.TrimSpace(cmd.Short) + hdr
		}
	}
}

// CheckFormatFlag resolves --format + --jq from cmd. Returns:
//   - (*FormatOptions{Mode:""}, nil)        flag not set; caller should call ResolveDefault
//   - (*FormatOptions{Mode:v,JQ:q}, nil)    valid values
//   - (nil, *FlagError)                     invalid --format, or --jq with explicit --format text
//   - (nil, input.invalid_argument error)   --jq does not parse (exit 5)
//
// --jq with --format unset is accepted: ResolveDefault and FromEnv promote a
// text default to FormatJSON so the filter has somewhere to apply.
func CheckFormatFlag(cmd *cobra.Command) (*FormatOptions, error) {
	fopts := &FormatOptions{}
	if f := cmd.Flags().Lookup("format"); f != nil {
		v := f.Value.String()
		switch v {
		case "":
			// unset; caller calls ResolveDefault
		case "text", "json", "ndjson":
			fopts.Mode = FormatMode(v)
		default:
			return nil, NewFlagError(fmt.Errorf("invalid --format %q: must be text | json | ndjson", v))
		}
	}
	if f := cmd.Flags().Lookup("jq"); f != nil {
		fopts.JQ = f.Value.String()
	}
	// --jq only meaningful for JSON-shaped output. Reject the explicit
	// `--format text --jq ...` combination; with --format unset, ResolveDefault
	// and FromEnv promote a text default to JSON.
	if fopts.JQ != "" && fopts.Mode == FormatText {
		return nil, NewFlagError(errors.New("--jq requires --format json|ndjson"))
	}
	// Parse now so a malformed --jq exits 5 before any request, not after a full --paginate fetch.
	if fopts.JQ != "" {
		if _, err := format.ParseJQ(fopts.JQ); err != nil {
			return nil, mapJQError(err)
		}
	}
	return fopts, nil
}

// WantsJSON reports whether the resolved mode is JSON or NDJSON. Used by
// callers to choose between the JSON emit path and text rendering.
func (o *FormatOptions) WantsJSON() bool {
	return o.Mode == FormatJSON || o.Mode == FormatNDJSON
}

// Emit serializes data wrapped in the success envelope (FormatJSON
// path) or as bare NDJSON lines (FormatNDJSON path). meta is optional
// (pass nil for mutation commands without batch counts).
//
// FormatJSON path: envelope is {ok:true, data:..., meta?:..., profile?:...}.
// Indent is determined by o.TTY (populated by ResolveDefault).
// When o.JQ is set, jq evaluates against the full envelope JSON, so users
// project with ".data[]", ".meta.count", etc.
//
// FormatNDJSON path: emits one bare JSON object per line (no envelope).
// Matches the NDJSON event-passthrough contract used by streaming commands.
//
// FormatText path returns an error so a missed dispatch surfaces loudly.
func (o *FormatOptions) Emit(w io.Writer, data any, meta *output.Meta) error {
	switch o.Mode {
	case FormatJSON:
		if o.JQ != "" {
			return mapJQError(format.WriteJSONFiltered(w, output.NewEnvelope(data, meta, globalProfile), nil, o.JQ))
		}
		return output.WriteEnvelope(w, data, meta, o.TTY, globalProfile)
	case FormatNDJSON:
		if o.JQ != "" {
			return mapJQError(format.WriteJSONFiltered(w, data, nil, o.JQ))
		}
		return format.WriteNDJSON(w, data)
	case FormatText:
		return fmt.Errorf("FormatOptions.Emit: cannot emit text mode as JSON; caller must render human-readable separately")
	default:
		return fmt.Errorf("FormatOptions.Emit: unknown mode %q", o.Mode)
	}
}

// mapJQError converts a failure rooted in the user-supplied --jq expression
// (format.JQError) into a typed input.invalid_argument error (exit 5) so agents
// fix the expression instead of treating a bad --jq as a CLI bug. Non-jq errors
// (e.g. an internal serialization fault) pass through unchanged.
func mapJQError(err error) error {
	if err == nil {
		return nil
	}
	var jqe *format.JQError
	if errors.As(err, &jqe) {
		return NewError(CodeInputInvalidArgument, jqe.Error()).
			WithHint("invalid --jq expression; see https://jqlang.github.io/jq/manual/")
	}
	return err
}

// ResolveDefault fills in Mode when the caller has not explicitly set it:
//   - Mode comes from ConfiguredDefaultMode (FormatJSON unless configured), regardless of TTY
//   - --jq promotes a text default to FormatJSON, since the filter needs JSON
//   - TTY only affects the indent decision (auto-indent in TTY; compact in pipe)
//   - For human-readable rendering, pass --format text explicitly
//
// JSON-always (not a TTY switch to text on a terminal) is deliberate: an
// agent-first CLI values output predictability over terminal ergonomics, so
// the default never depends on whether stdout is a TTY. Humans opt into
// human-readable output with `--format text`.
func (o *FormatOptions) ResolveDefault(tty bool) {
	o.TTY = tty
	// --format was already set on o.Mode by CheckFormatFlag.
	if o.Mode == "" {
		o.Mode = ConfiguredDefaultMode()
	}
	o.jqImpliesJSON()
}

// jqImpliesJSON promotes text to JSON when --jq is set. Text reaching here is
// always a default: CheckFormatFlag rejects an explicit --format text --jq.
func (o *FormatOptions) jqImpliesJSON() {
	if o.JQ != "" && o.Mode == FormatText {
		o.Mode = FormatJSON
	}
}

// ConfiguredDefaultMode is the mode used when --format is unset:
// AIKB_FORMAT, else config.yaml defaults.format, else DefaultFormatMode.
// Invalid values at either level are skipped.
func ConfiguredDefaultMode() FormatMode {
	if m, ok := parseFormatMode(os.Getenv("AIKB_FORMAT")); ok {
		return m
	}
	if cfg, err := config.Load(); err == nil {
		if m, ok := parseFormatMode(cfg.Defaults.Format); ok {
			return m
		}
	}
	return DefaultFormatMode
}

func parseFormatMode(v string) (FormatMode, bool) {
	switch m := FormatMode(strings.ToLower(v)); m {
	case FormatText, FormatJSON, FormatNDJSON:
		return m, true
	}
	return "", false
}

// FromEnv applies AIKB_FORMAT when Mode hasn't been set by --format, without
// the config.yaml default. For commands such as `auth token` whose own
// default replaces the CLI-wide one. Invalid values are ignored, and --jq
// promotes AIKB_FORMAT=text to JSON as in ResolveDefault.
func (o *FormatOptions) FromEnv() {
	if o.Mode != "" {
		return
	}
	if m, ok := parseFormatMode(os.Getenv("AIKB_FORMAT")); ok {
		o.Mode = m
	}
	o.jqImpliesJSON()
}

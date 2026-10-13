package compat

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Level is the client-server version compat level.
type Level int

// Levels returned by Compat.
const (
	OK       Level = iota // compatible
	SoftWarn              // server outdated; some new features may be unavailable
)

func (l Level) String() string {
	switch l {
	case OK:
		return "ok"
	case SoftWarn:
		return "soft_warn"
	}
	return "unknown"
}

// Compat compares CalVer client/server versions (YYYY.M.D[.MICRO]). A release
// date says nothing about wire compatibility, so version skew alone never fails:
//
//	client <= server     -> OK
//	client >  server     -> SoftWarn (some new features may be unavailable)
//	unparseable string   -> OK (fail-open, do not block)
//	"(unknown)" / ""     -> OK (dev build / server field missing)
func Compat(serverVer, cliVer string) (Level, string) {
	s, ok := parseCalVer(serverVer)
	if !ok {
		return OK, ""
	}
	c, ok := parseCalVer(cliVer)
	if !ok {
		return OK, ""
	}
	if slices.Compare(c[:], s[:]) > 0 {
		return SoftWarn, fmt.Sprintf("server is older (server %s, client %s); some new features may be unavailable", serverVer, cliVer)
	}
	return OK, ""
}

// parseCalVer reads "YYYY.M.D" or "YYYY.M.D.MICRO" (a missing MICRO is 0),
// tolerating a leading "v" (tags) and a "-"/"+" suffix (`git describe`).
func parseCalVer(s string) (v [4]int, ok bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 && len(parts) != 4 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

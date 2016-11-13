package protocol

import (
	"fmt"
	"strings"
)

// Capabilities is the set of protocol capabilities supported by this
// implementation.
var Capabilities = CapList{
	"delete-refs":        true,
	"multi_ack_detailed": true,
	"no-done":            true,
	"ofs-delta":          true,
	"report-status":      true,
}

// A CapList represents a set of Git protocol capabilities.
type CapList map[string]bool

// sub returns a capability listing containing those capabilities in c
// that are not in d.
func (c CapList) sub(d CapList) CapList {
	e := make(CapList)
	for cap, ok := range c {
		if ok && !d[cap] {
			e[cap] = true
		}
	}
	return e
}

// String returns the capabilities in c joined by spaces.
func (c CapList) String() string {
	capList := make([]string, len(c))
	i := 0
	for cp, ok := range c {
		if ok {
			capList[i] = cp
		}
		i++
	}
	return strings.Join(capList[:i], " ")
}

// Scan is a support routine for fmt.Scanner.  The format verb is
// ignored; Scan always tries to extract a list of space-separated
// capability keywords from ss.
func (c *CapList) Scan(ss fmt.ScanState, verb rune) error {
	if *c == nil {
		*c = make(CapList)
	}
	for {
		tok, err := ss.Token(true, nil)
		// ss.Token (apparently?) indicates end-of-input by
		// returning [], nil, not [], io.EOF.
		if len(tok) == 0 {
			return nil
		} else if err != nil {
			return err
		}
		(*c)[string(tok)] = true
	}
	return nil
}

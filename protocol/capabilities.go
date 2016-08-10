package protocol

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/lxr/go.git-scm/pktline"
)

// Capabilities is the set of protocol capabilities supported by this
// implementation.
var Capabilities = CapList{
	"report-status":      true,
	"delete-refs":        true,
	"ofs-delta":          true,
	"multi_ack_detailed": true,
}

// A CapList represents a set of Git protocol capabilities.
type CapList map[string]bool

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

// ParseCapList parses a whitespace-separated list of capabilities.
func ParseCapList(s string) CapList {
	c := make(CapList)
	for _, cp := range strings.Fields(s) {
		c[cp] = true
	}
	return c
}

// diff returns the set of capabilities that are in a but not in b.
func diff(a, b CapList) CapList {
	c := make(CapList)
	for cp, ok := range a {
		if ok && !b[cp] {
			c[cp] = true
		}
	}
	return c
}

// scanCmds scans a list of pkt-line records to l, which must be of type
// *[]T, where *T can be fmt.Scanned into.  For the first pkt-line,
// anything left over from the scan is interpreted as a list of protocol
// capabilities and returned as c.
func scanCmds(pktr *pktline.Reader, l interface{}) (c CapList, err error) {
	if err := pktr.Next(); err != nil {
		return nil, err
	}
	v := reflect.ValueOf(l).Elem()
	t := v.Type().Elem()
	var msg []byte
	for {
		msg, err = pktr.ReadMsg()
		switch {
		case err == io.EOF:
			return c, nil
		case err != nil:
			return c, err
		}
		buf := bytes.NewBuffer(msg)
		x := reflect.New(t)
		if _, err := fmt.Fscan(buf, x.Interface()); err != nil {
			return nil, err
		}
		v.Set(reflect.Append(v, x.Elem()))
		if c == nil {
			c = ParseCapList(buf.String())
			if d := diff(c, Capabilities); len(d) != 0 {
				return nil, fmt.Errorf("unrecognized capabilities: %s", d)
			}
		}
	}
}

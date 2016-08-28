package protocol

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/packfile"
	"github.com/lxr/go.git-scm/pktline"
	"github.com/lxr/go.git-scm/repository"
)

// A refName is a string representing the name of a Git ref.  Its Scan
// method refuses to scan certain ill-formed refs, but not all.
type refName string

func (r *refName) Scan(ss fmt.ScanState, verb rune) error {
	tok, err := ss.Token(true, func(r rune) bool {
		return r >= 0x20 && !strings.ContainsRune(" *:?[^~", r)
	})
	if err != nil {
		return err
	}
	*r = refName(tok)
	return nil
}

// BUG(lor): ReceivePack does not understand push certificates or
// shallow refs.

// ReceivePack reads a pkt-line stream of ref update commands and a
// packfile from r and updates repo accordingly.  If the report-status
// capability is set in r, the progress of the task is written in
// pkt-lines to w.  ReceivePack returns a non-nil error only if it fails
// to read the ref update commands; failures to unpack the packfile or
// update individual refs are merely logged to w.
func ReceivePack(repo repository.Interface, w io.Writer, r io.Reader) error {
	type receiveCmd struct {
		oldID object.ID
		newID object.ID
		name  refName
	}

	pktr := pktline.NewReader(r)
	deleteCommandsOnly := true
	var cmds []receiveCmd
	var caps CapList
	for {
		var cmd receiveCmd
		if n, err := fmtLscanf(pktr, "%s %s %s\x00%s",
			&cmd.oldID, &cmd.newID, &cmd.name, &caps); err == io.EOF {
			break
		} else if n < 3 {
			return err
		}
		cmds = append(cmds, cmd)
		if cmd.newID != object.ZeroID {
			deleteCommandsOnly = false
		}
	}
	if d := caps.diff(Capabilities); len(d) > 0 {
		return fmt.Errorf("unrecognized capabilities: %s", d)
	}

	if !caps["report-status"] {
		w = ioutil.Discard
	}
	pktw := pktline.NewWriter(w)

	var err error
	if !deleteCommandsOnly {
		err = unpack(repo, r)
	}
	if err == nil {
		// Rather sillily, "unpack ok" is expected to be sent
		// event if no packfile was actually unpacked.
		fmtLprintf(pktw, "unpack ok\n")
	} else {
		fmtLprintf(pktw, "unpack %s\n", err)
	}

	for _, c := range cmds {
		if err := repository.UpdateRef(repo, string(c.name), c.oldID, c.newID); err != nil {
			fmtLprintf(pktw, "ng %s %s\n", c.name, err)
			continue
		}
		fmtLprintf(pktw, "ok %s\n", c.name)
	}

	pktw.Flush()
	return nil
}

// unpack reads a packfile from r (with repo as reference) and stores
// all its objects in repo.
func unpack(repo repository.Interface, r io.Reader) error {
	pfr, err := packfile.NewReader(r, repo)
	if err != nil {
		return err
	}
	for pfr.Len() > 0 {
		obj, err := pfr.ReadObject()
		if err != nil {
			return err
		}
		_, err = repo.PutObject(obj)
		if err != nil {
			return err
		}
	}
	return pfr.Close()
}

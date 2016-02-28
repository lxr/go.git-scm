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

type receiveCmd struct {
	oldID object.ID
	newID object.ID
	name  string
}

func (c *receiveCmd) Scan(ss fmt.ScanState, verb rune) error {
	_, err := fmt.Fscan(ss, &c.oldID, &c.newID, &c.name)
	c.name = strings.TrimSuffix(c.name, "\x00")
	return err
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
	pktr := pktline.NewReader(r)
	var cmds []receiveCmd
	caps, err := scanCmds(pktr, &cmds)
	if err != nil {
		return err
	}

	if !caps["report-status"] {
		w = ioutil.Discard
	}
	pktw := pktline.NewWriter(w)
	defer pktw.Flush()

	if !onlyDeleteCommandsIn(cmds) {
		err = unpack(repo, r)
	}
	if err == nil {
		fmt.Fprintf(pktw, "unpack ok\n")
	} else {
		fmt.Fprintf(pktw, "unpack %s\n", err)
	}

	for _, c := range cmds {
		if err := repository.UpdateRef(repo, c.name, c.oldID, c.newID); err != nil {
			fmt.Fprintf(pktw, "ng %s %s\n", c.name, err)
			continue
		}
		fmt.Fprintf(pktw, "ok %s\n", c.name)
	}
	return nil
}

func onlyDeleteCommandsIn(cmds []receiveCmd) bool {
	for _, cmd := range cmds {
		if cmd.newID != object.ZeroID {
			return false
		}
	}
	return true
}

func unpack(repo repository.Interface, r io.Reader) error {
	pfr, err := packfile.NewReader(r)
	if err != nil {
		return err
	}
	for pfr.Len() > 0 {
		obj, err := pfr.Read()
		if err != nil {
			return err
		}
		_, err = repo.PutObject(obj)
		if err != nil {
			return err
		}
	}
	return nil
}

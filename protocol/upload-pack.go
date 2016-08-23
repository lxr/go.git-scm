package protocol

import (
	"fmt"
	"io"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/packfile"
	"github.com/lxr/go.git-scm/pktline"
	"github.com/lxr/go.git-scm/repository"
)

// BUG(lor): UploadPack does not understand the
// shallow and deepen commands.

// UploadPack reads from r a pkt-line stream of refs that the client
// wants and has and writes a packfile bridging the two sets to w.
func UploadPack(repo repository.Interface, w io.Writer, r io.Reader) error {
	pktr := pktline.NewReader(r)
	var want []object.ID
	var caps CapList
	if err := pktr.Next(); err != nil {
		return err
	}
	for {
		var id object.ID
		str, err := pktr.ReadMsgString()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		} else if n, err := fmt.Sscanf(str, "want %s %s", &id, &caps); n < 1 {
			return err
		}
		want = append(want, id)
	}
	if d := caps.diff(Capabilities); len(d) > 0 {
		return fmt.Errorf("unrecognized capabilities: %s", d)
	}

	pktw := pktline.NewWriter(w)
	common, err := negotiate(repo, pktw, pktr, caps["multi_ack_detailed"])
	switch {
	case err == io.EOF:
		// If pktr does not end with a done line, the client
		// will send the rest of its haves in a separate
		// request.  We quit early and don't send a packfile
		// in that case.
		return nil
	case err != nil:
		return err
	}

	objs, err := repository.Negotiate(repo, want, common)
	if err != nil {
		return err
	}
	return writePack(w, objs)
}

// negotiate loops over the substreams of pktr until it encounters
// either a pkt-line consisting of "done\n" or an error, and returns the
// list of all object IDs given in "have" pkt-lines that exist in repo.
// ACK and NAK lines are written to pktw depending on the multiAck mode.
// If pktr ends before a done line is received, the error is io.EOF.
func negotiate(repo repository.Interface, pktw *pktline.Writer, pktr *pktline.Reader, multiAck bool) (common []object.ID, err error) {
	var (
		ok   bool
		msg  string
		id   object.ID
		last object.ID
	)
	if err = pktr.Next(); err != nil {
		return
	}
	for {
		msg, err = pktr.ReadMsgString()
		switch {
		case err == io.EOF:
			if len(common) == 0 || multiAck {
				fmt.Fprintf(pktw, "NAK\n")
			}
			if err = pktr.Next(); err != nil {
				return
			}
			continue
		case err != nil:
			return
		case msg == "done\n":
			if len(common) == 0 {
				fmt.Fprintf(pktw, "NAK\n")
			} else if multiAck {
				fmt.Fprintf(pktw, "ACK %s\n", last)
			}
			return
		}

		_, err = fmt.Sscanf(msg, "have %s\n", &id)
		if err != nil {
			return
		}
		ok, err = repository.HasObject(repo, id)
		if err != nil {
			return
		}
		if ok {
			if multiAck {
				fmt.Fprintf(pktw, "ACK %s common\n", id)
			} else if len(common) == 0 {
				fmt.Fprintf(pktw, "ACK %s\n", id)
			}
			last = id
			common = append(common, last)
		}
	}
}

func writePack(w io.Writer, objs []object.Interface) error {
	pfw, err := packfile.NewWriter(w, int64(len(objs)))
	if err != nil {
		return err
	}
	for _, obj := range objs {
		if err := pfw.Write(obj); err != nil {
			return err
		}
	}
	return pfw.Close()
}

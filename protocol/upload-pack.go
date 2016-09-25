package protocol

import (
	"fmt"
	"io"
	"sort"

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
	for {
		var id object.ID
		if n, err := fmtLscanf(pktr, "want %s %s", &id, &caps); err == io.EOF {
			break
		} else if n < 1 {
			return err
		}
		want = append(want, id)
	}
	if d := caps.diff(Capabilities); len(d) > 0 {
		return fmt.Errorf("unrecognized capabilities: %s", d)
	}

	pktw := pktline.NewWriter(w)
	pktr.Next()
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
	for {
		msg, err = pktr.ReadLine()
		switch {
		case err == io.EOF:
			if len(common) == 0 || multiAck {
				fmtLprintf(pktw, "NAK\n")
			}
			pktr.Next()
			continue
		case err != nil:
			return
		case msg == "done\n":
			if len(common) == 0 {
				fmtLprintf(pktw, "NAK\n")
			} else if multiAck {
				fmtLprintf(pktw, "ACK %s\n", last)
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
				fmtLprintf(pktw, "ACK %s common\n", id)
			} else if len(common) == 0 {
				fmtLprintf(pktw, "ACK %s\n", id)
			}
			last = id
			common = append(common, last)
		}
	}
}

// objectSizeOf returns an approximation of the given object's binary
// representation size.  It returns -1 if the object is not one of the
// standard Git types.
func objectSizeOf(obj object.Interface) int {
	switch obj := obj.(type) {
	case *object.Commit:
		n := 6 + 40
		for range obj.Parent {
			n += 8 + 40
		}
		n += 28 + len(obj.Author.Name) + len(obj.Author.Email)
		n += 31 + len(obj.Committer.Name) + len(obj.Committer.Email)
		n += 1 + len(obj.Message)
		return n
	case object.Tree:
		n := 0
		for filename := range obj {
			n += 28 + len(filename)
		}
		return n
	case *object.Blob:
		return len(*obj)
	case *object.Tag:
		n := 8 + 40
		n += 6 + len(obj.Type.String())
		n += 5 + len(obj.Tag)
		n += 28 + len(obj.Tagger.Name) + len(obj.Tagger.Email)
		n += 1 + len(obj.Message)
		return n
	default:
		return -1
	}
}

// objSlice is a wrapper type for sorting Git object slices by type
// and size.  Packfiles written in this order compress better.
type objSlice []object.Interface

func (objs objSlice) Len() int {
	return len(objs)
}

func (objs objSlice) Less(i, j int) bool {
	a := objs[i]
	b := objs[j]
	aType := object.TypeOf(a)
	bType := object.TypeOf(b)
	switch {
	case aType < bType:
		return true
	case aType > bType:
		return false
	default: // aType == bType
		return objectSizeOf(a) >= objectSizeOf(b)
	}
}

func (objs objSlice) Swap(i, j int) {
	objs[i], objs[j] = objs[j], objs[i]
}

func writePack(w io.Writer, objs []object.Interface) error {
	pfw, err := packfile.NewWriter(w, int64(len(objs)))
	if err != nil {
		return err
	}
	sort.Sort(objSlice(objs))
	for _, obj := range objs {
		if err := pfw.WriteObject(obj); err != nil {
			return err
		}
	}
	return pfw.Close()
}

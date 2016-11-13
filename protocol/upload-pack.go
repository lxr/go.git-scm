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

// BUG(lor): UploadPack's support for non-multi_ack_detailed operation
// is experimental.

// UploadPack reads from r a pkt-line stream of refs that the client
// wants and has and writes a packfile bridging the two sets to w.
func UploadPack(repo repository.Interface, w io.Writer, r io.Reader) error {
	pktr := pktline.NewReader(r)
	want := make(map[object.ID]bool)
	var start, end []object.ID
	var caps CapList
	for {
		var id object.ID
		if n, err := fmtLscanf(pktr, "want %s %s", &id, &caps); err == io.EOF {
			break
		} else if n < 1 {
			return err
		}
		want[id] = true
		start = append(start, id)
	}
	if len(want) == 0 {
		return nil
	}
	if d := caps.sub(Capabilities); len(d) > 0 {
		return fmt.Errorf("unrecognized capabilities: %s", d)
	}

	pktw := pktline.NewWriter(w)
	for {
		pktr.Next()
		have, err := readHaveLines(pktr)
		if len(have) == 0 && err == io.ErrUnexpectedEOF {
			return nil
		} else if err != io.EOF && err != nil {
			return err
		}
		// XXX(lor): This is potentially a lot of repository
		// walking.  Can it be made any cheaper?
		for wantID := range want {
			err := repository.Walk(repo, []object.ID{wantID}, nil, func(id object.ID, obj object.Interface, err error) error {
				if err != nil {
					return err
				}
				// BUG(lor): UploadPack can neglect to
				// report common objects as common if
				// they are parents of another common
				// object, as the repository traversal
				// never proceeds past a common object.
				// As have lines are sent in reverse
				// chronological order, this is actually
				// very common.
				if _, ok := have[id]; ok {
					delete(want, wantID)
					have[id] = true
					end = append(end, id)
					return repository.SkipObject
				}
				// We assume that wants and haves never
				// point at trees, blobs or Git
				// submodules, so avoid recursing deeper
				// into non-commit and non-tag objects.
				switch obj.(type) {
				case *object.Commit, *object.Tag:
					return nil
				default:
					return repository.SkipObject
				}
			})
			if err != nil {
				return err
			}
		}
		if caps["multi_ack_detailed"] {
			for haveID, common := range have {
				if common {
					fmtLprintf(pktw, "ACK %s common\n", haveID)
				}
			}
			if len(want) == 0 {
				fmtLprintf(pktw, "ACK %s ready\n", end[len(end)-1])
				if caps["no-done"] && err == nil {
					// XXX(lor): The protocol
					// capability documentation
					// says, "the sender is free to
					// immediately send a pack
					// following its first 'ACK
					// obj-id ready' message", but
					// what it really means is that
					// the sender is free to behave
					// as if a "done" had been sent
					// immediately after the current
					// have block's flush-pkt.
					fmtLprintf(pktw, "NAK\n")
					err = io.EOF
				}
			}
		}
		// BUG(lor): When not in multi_ack_detailed mode,
		// UploadPack ACKs the last of the common commits
		// identifies, not the first one.
		if len(end) > 0 && (err == io.EOF) == caps["multi_ack_detailed"] {
			fmtLprintf(pktw, "ACK %s\n", end[len(end)-1])
		} else {
			fmtLprintf(pktw, "NAK\n")
		}
		if err == io.EOF {
			break
		}
	}

	var hdrs objHeaderSlice
	err := repository.Walk(repo, start, end, func(id object.ID, obj object.Interface, err error) error {
		if err != nil {
			return err
		}
		hdrs = append(hdrs, objHeader{
			ID:   id,
			Type: object.TypeOf(obj),
			Size: objectSizeOf(obj),
		})
		return nil
	})
	if err != nil {
		return err
	}
	sort.Sort(hdrs)

	pfw, err := packfile.NewWriter(w, int64(len(hdrs)))
	if err != nil {
		return err
	}
	for _, hdr := range hdrs {
		obj, err := repo.GetObject(hdr.ID)
		if err != nil {
			return err
		}
		if err := pfw.WriteObject(obj); err != nil {
			return err
		}
	}
	return pfw.Close()
}

// readHaveLines reads a flush-pkt-or-"done"-terminated sequence of
// "have obj-id" lines from pktr and returns the obj-ids as a boolean
// map.  The map values are false, so as to allow client code to mark
// the actually common objects as true.  If the sequence ends in a
// flush-pkt, the error is nil; if it ends in a "done", the error is
// io.EOF.  A sequence ending in "done" is not necessarily empty, so
// client code needs to remember to process the map even on an io.EOF
// error.
func readHaveLines(pktr *pktline.Reader) (map[object.ID]bool, error) {
	have := make(map[object.ID]bool)
	for {
		var cmd string
		var id object.ID
		n, err := fmtLscanf(pktr, "%s %s", &cmd, &id)
		switch {
		case n == 0 && err == io.EOF: // flush-pkt
			return have, nil
		case n == 1 && cmd == "done":
			return have, io.EOF
		case n == 2 && cmd == "have":
			have[id] = false
		case n == 2 && cmd != "have":
			return have, fmt.Errorf("bad command: %q", cmd)
		default:
			return have, err
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
	case *object.Tree:
		n := 0
		for filename := range *obj {
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

// An objHeader contains the information necessary for sorting a Git
// object for good delta compression.
type objHeader struct {
	ID   object.ID
	Type object.Type
	Size int
}

// objHeaderSlice implements the compression-optimized total order:
// together by type (ascending in this implementation) and descending
// by size.
type objHeaderSlice []objHeader

func (hdrs objHeaderSlice) Len() int {
	return len(hdrs)
}

func (hdrs objHeaderSlice) Less(i, j int) bool {
	switch {
	case hdrs[i].Type < hdrs[j].Type:
		return true
	case hdrs[i].Type > hdrs[j].Type:
		return false
	default: // hdrs[i].Type == hdrs[j].Type
		return hdrs[i].Size >= hdrs[j].Size
	}
}

func (hdrs objHeaderSlice) Swap(i, j int) {
	hdrs[i], hdrs[j] = hdrs[j], hdrs[i]
}

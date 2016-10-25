package protocol

import (
	"io"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/pktline"
	"github.com/lxr/go.git-scm/repository"
)

// BUG(lor): AdvertiseRefs does not properly mark shallow references as
// such.

// BUG(lor): AdvertiseRefs includes refs that point to nonexistent
// objects.

// AdvertiseRefs writes Capabilities and a list of available refs in
// repo to w in pkt-line format.  It returns a non-nil error only if it
// could not list the references; in particular errors writing to w or
// peeling annotated tags are ignored.
func AdvertiseRefs(repo repository.Interface, w io.Writer) error {
	names, ids, err := repo.ListRefs()
	if err != nil {
		return err
	}
	pktw := pktline.NewWriter(w)
	HEAD, _ := repo.GetHEAD()
	if id, err := repo.GetRef(HEAD); err == nil {
		names = append([]string{"HEAD"}, names...)
		ids = append([]object.ID{id}, ids...)
	}
	if len(names) == 0 {
		names = []string{"capabilities^{}"}
		ids = []object.ID{object.ZeroID}
	}
	for i := range names {
		name, id := names[i], ids[i]
		if i == 0 {
			fmtLprintf(pktw, "%s %s\x00%s\n", id, name, Capabilities)
		} else {
			fmtLprintf(pktw, "%s %s\n", id, name)
		}
		if tag, _, err := repository.GetTag(repo, id); err == nil {
			fmtLprintf(pktw, "%s %s^{}\n", tag.Object, name)
		}
	}
	pktw.Flush()
	return nil
}

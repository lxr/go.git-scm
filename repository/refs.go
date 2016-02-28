package repository

import (
	"fmt"

	"github.com/lxr/go.git-scm/object"
)

// UpdateRef points the named ref at newID only if it pointed at oldID
// and the object referenced by newID exists in the Git database.
// If oldID is object.ZeroID, the ref is created if it does not exist;
// if newID is object.ZeroID, the ref is deleted if oldID matches.
// If both are object.ZeroID, UpdateRef ensures that the named ref does
// not exist.
func UpdateRef(r Interface, name string, oldID, newID object.ID) error {
	id, err := r.GetRef(name)
	switch {
	case err == ErrNotExist && oldID == object.ZeroID:
		// do nothing
	case err != nil:
		return err
	case id != oldID:
		return fmt.Errorf("old-id mismatch")
	}
	err = nil
	ok := true
	if newID != object.ZeroID {
		ok, err = HasObject(r, newID)
	} else if oldID == object.ZeroID {
		return nil
	} else {
		return r.DelRef(name)
	}
	switch {
	case err != nil:
		return err
	case !ok:
		return fmt.Errorf("new-id refers to a nonexistent object")
	}
	return r.SetRef(name, newID)
}

// BUG(lor): UpdateRef is not atomic.

// FindRef disambiguates an abbreviated refname according to the
// gitrevisions(7) rules.  Symbolic refs are not resolved.
func FindRef(r Interface, name string) (object.ID, error) {
	for _, format := range findRefList {
		if id, err := r.GetRef(fmt.Sprintf(format, name)); err == nil {
			return id, err
		}
	}
	return object.ZeroID, ErrNotExist
}

var findRefList = []string{
	"refs/%s",
	"refs/tags/%s",
	"refs/heads/%s",
	"refs/remotes/%s",
}

package repository

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/lxr/go.git-scm/object"
)

// HasObject returns true if and only if an object with the given ID
// exists in the repository.
func HasObject(r Interface, id object.ID) (bool, error) {
	_, err := r.GetObject(id)
	switch err {
	case nil:
		return true, nil
	case ErrObjectNotExist:
		return false, nil
	default:
		return false, err
	}
}

// GetCommit recursively dereferences the given ID to a commit object
// and returns its ID.  If an object cannot be dereferenced into a
// commit, GetCommit returns its ID and an *object.TypeError containing
// the object.
func GetCommit(r Interface, id object.ID) (*object.Commit, object.ID, error) {
	obj, err := r.GetObject(id)
	if err != nil {
		return nil, id, err
	}
	switch obj := obj.(type) {
	case *object.Commit:
		return obj, id, nil
	case *object.Tag:
		switch obj.Type {
		case object.TypeCommit, object.TypeTag:
			return GetCommit(r, obj.Object)
		default:
			return nil, id, &object.TypeError{obj}
		}
	default:
		return nil, id, &object.TypeError{obj}
	}
}

// GetTag recursively dereferences the given ID to a tag object that
// points to a non-tag object.  If an object cannot be dereferenced into
// a tag, GetTag returns its ID and an *object.TypeError containing the
// retrieved object.
func GetTag(r Interface, id object.ID) (*object.Tag, object.ID, error) {
	obj, err := r.GetObject(id)
	if err != nil {
		return nil, id, err
	}
	tag, ok := obj.(*object.Tag)
	switch {
	case !ok:
		return nil, id, &object.TypeError{obj}
	case tag.Type == object.TypeTag:
		return GetTag(r, tag.Object)
	default:
		return tag, id, nil
	}
}

// GetTree recursively dereferences the given ID to a tree object and
// returns its ID.  If an object cannot be dereferenced into a tree,
// GetTree returns its ID and an *object.TypeError containing the
// object.
func GetTree(r Interface, id object.ID) (*object.Tree, object.ID, error) {
	obj, err := r.GetObject(id)
	if err != nil {
		return nil, id, err
	}
	switch obj := obj.(type) {
	case *object.Tree:
		return obj, id, nil
	case *object.Commit:
		return GetTree(r, obj.Tree)
	case *object.Tag:
		switch obj.Type {
		case object.TypeTree, object.TypeCommit, object.TypeTag:
			return GetTree(r, obj.Object)
		default:
			return nil, id, &object.TypeError{obj}
		}
	default:
		return nil, id, &object.TypeError{obj}
	}
}

// GetPath retrieves the object with the given filename in the tree
// hierarchy rooted at the given ID.  The ID of the object is also
// retrieved.  The root ID may point to a tree, commit or tag object.
// If name resolves to "/", GetPath returns the first tree object
// derived from the root ID.  If a path component is missing from
// a tree object during the walk, GetPath returns the tree object and
// its ID and an error designating the missing component.  If an
// intermediate object cannot be dereferenced into a tree, GetPath
// returns its ID and an *object.TypeError containing the object.
func GetPath(r Interface, id object.ID, name string) (object.Interface, object.ID, error) {
	tree, id, err := GetTree(r, id)
	if err != nil {
		return nil, id, err
	}
	comps := strings.Split(path.Clean("/" + name)[1:], "/")
	if comps[0] == "" {
		return tree, id, nil
	}
	obj := object.Interface(tree)
	for _, comp := range comps {
		switch obj := obj.(type) {
		case *object.Tree:
			tree = obj
		case *object.Commit:
			tree, id, err = GetTree(r, obj.Tree)
			if err != nil {
				return nil, id, err
			}
		// NOTE(lor): A tag object can potentially be
		// dereferenced into a tree, but a tree hierarchy
		// shouldn't contain them, so GetPath returns an
		// *object.TypeError if a tag object is encountered
		// while walking the tree hierarchy.
		default:
			return nil, id, &object.TypeError{obj}
		}
		ti, ok := (*tree)[comp]
		if !ok {
			return tree, id, fmt.Errorf("no such tree entry: %s", comp)
		}
		obj, err = r.GetObject(ti.Object)
		if err != nil {
			return nil, id, err
		}
	}
	return obj, id, nil
}

// SkipObject is a special value used by WalkFunc to indicate that the
// repository walk should skip the subgraph rooted at the object.
var SkipObject = errors.New("skip this object")

// WalkFunc is the callback function type for Walk.  It takes as its
// arguments the object's ID and contents and any possible error in
// retrieving the object from the repository.  If err is non-nil, obj
// is undefined, and the WalkFunc should return either SkipObject or a
// non-nil error.
type WalkFunc func(id object.ID, obj object.Interface, err error) error

// Walk walks the repository graph from the start objects (inclusive)
// to the end objects (exclusive), calling walkFn once for each
// encountered object.  Walk ends at and returns the first non-nil error
// returned by walkFn, unless the error is SkipObject, in which case
// Walk continues without searching the subgraph rooted at the current
// object.
//
// Walk traverses the repository in depth-first order.  Non-standard Git
// objects are treated as having no references for the purposes of the
// traversal; they are still passed to walkFn.
func Walk(r Interface, start, end []object.ID, walkFn WalkFunc) error {
	visited := make(map[object.ID]bool)
	for _, id := range end {
		visited[id] = true
	}
	pending := make([]object.ID, len(start))
	copy(pending, start)
	for len(pending) > 0 {
		var id object.ID
		n := len(pending) - 1
		id, pending = pending[n], pending[:n]
		if visited[id] {
			continue
		}
		visited[id] = true

		obj, err := r.GetObject(id)
		err = walkFn(id, obj, err)
		if err == SkipObject {
			continue
		} else if err != nil {
			return err
		}

		switch obj := obj.(type) {
		case *object.Commit:
			pending = append(pending, obj.Tree)
			for _, parent := range obj.Parent {
				pending = append(pending, parent)
			}
		case *object.Tree:
			for _, ti := range *obj {
				pending = append(pending, ti.Object)
			}
		case *object.Blob:
			// a blob holds no references
		case *object.Tag:
			pending = append(pending, obj.Object)
		}
	}
	return nil
}

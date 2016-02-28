package repository

import (
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
	case ErrNotExist:
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
func GetTree(r Interface, id object.ID) (object.Tree, object.ID, error) {
	obj, err := r.GetObject(id)
	if err != nil {
		return nil, id, err
	}
	switch obj := obj.(type) {
	case object.Tree:
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
		case object.Tree:
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
		ti, ok := tree[comp]
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

// Negotiate performs a breadth-first traversal of the repository
// starting from the wanted objects and ending at the had objects (or
// Git blobs, which reference no other objects).  It returns a slice
// of all the traversed objects.  The slice contains all objects in
// want but none of the ones in have.  If a non-standard Git object is
// encountered during the traversal, Negotiate returns an
// *object.TypeError containing it.
func Negotiate(r Interface, want []object.ID, have []object.ID) ([]object.Interface, error) {
	has := make(map[object.ID]object.Interface)
	for _, id := range have {
		has[id] = nil
	}
	nhave := len(has)
	for len(want) > 0 {
		var id object.ID
		id, want = want[0], want[1:]
		if _, ok := has[id]; ok {
			continue
		}
		obj, err := r.GetObject(id)
		if err != nil {
			return nil, err
		}
		has[id] = obj
		switch obj := obj.(type) {
		case *object.Commit:
			want = append(want, obj.Tree)
			for _, parent := range obj.Parent {
				want = append(want, parent)
			}
		case object.Tree:
			for _, ti := range obj {
				want = append(want, ti.Object)
			}
		case *object.Blob:
			// a blob holds no references
		case *object.Tag:
			want = append(want, obj.Object)
		default:
			return nil, &object.TypeError{obj}
		}
	}
	res := make([]object.Interface, len(has)-nhave)
	i := 0
	for _, obj := range has {
		if obj != nil {
			res[i] = obj
			i++
		}
	}
	return res, nil
}

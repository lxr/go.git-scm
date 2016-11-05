// Package mem implements a main memory-backed Git repository.
package mem

import (
	"sort"
	"sync"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/repository"
)

// NewRepository initializes and returns a new in-memory Git repository.
func NewRepository() repository.Interface {
	return &repo{
		objects: make(map[object.ID]object.Interface),
		refs:    make(map[string]object.ID),
		HEAD:    "refs/heads/master",
	}
}

type repo struct {
	objectsLock sync.RWMutex
	objects     map[object.ID]object.Interface

	refsLock sync.RWMutex
	refs     map[string]object.ID

	HEADLock sync.RWMutex
	HEAD     string
}

func (r *repo) GetObject(id object.ID) (object.Interface, error) {
	r.objectsLock.RLock()
	defer r.objectsLock.RUnlock()
	obj, ok := r.objects[id]
	if !ok {
		return nil, repository.ErrObjectNotExist
	}
	return obj, nil
}

func (r *repo) PutObject(obj object.Interface) (object.ID, error) {
	id, err := object.Hash(obj)
	if err != nil {
		return id, err
	}
	r.objectsLock.Lock()
	defer r.objectsLock.Unlock()
	r.objects[id] = obj
	return id, nil
}

func (r *repo) GetRef(name string) (object.ID, error) {
	if !repository.IsValidRef(name) {
		return object.ZeroID, repository.ErrInvalidRef
	}
	r.refsLock.RLock()
	defer r.refsLock.RUnlock()
	id, ok := r.refs[name]
	if !ok {
		return id, repository.ErrRefNotExist
	}
	return id, nil
}

func (r *repo) UpdateRef(name string, oldID, newID object.ID) error {
	if !repository.IsValidRef(name) {
		return repository.ErrInvalidRef
	}

	r.refsLock.Lock()
	defer r.refsLock.Unlock()

	id := r.refs[name]
	if id != oldID {
		switch object.ZeroID {
		case id:
			return repository.ErrRefNotExist
		case oldID:
			return repository.ErrRefExist
		default:
			return repository.ErrRefMismatch
		}
	}

	switch newID {
	case object.ZeroID:
		// This is a no-op when r.refs[name] does not exist,
		// i.e. when id == oldID == object.ZeroID.
		delete(r.refs, name)
		return nil
	default:
		if _, err := r.GetObject(newID); err != nil {
			return err
		}
		r.refs[name] = newID
		return nil
	}
}

func (r *repo) ListRefs() ([]string, []object.ID, error) {
	r.refsLock.RLock()
	defer r.refsLock.RUnlock()
	names := make(sort.StringSlice, 0, len(r.refs))
	for name := range r.refs {
		names = append(names, name)
	}
	names.Sort()
	ids := make([]object.ID, len(names))
	for i, name := range names {
		ids[i] = r.refs[name]
	}
	return names, ids, nil
}

func (r *repo) GetHEAD() (string, error) {
	r.HEADLock.RLock()
	defer r.HEADLock.RUnlock()
	return r.HEAD, nil
}

func (r *repo) SetHEAD(name string) error {
	r.HEADLock.Lock()
	defer r.HEADLock.Unlock()
	r.HEAD = name
	return nil
}

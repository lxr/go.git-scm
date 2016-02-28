package appengine

import (
	"strings"

	"google.golang.org/appengine/datastore"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/repository"
)

func (r *repo) refKey(name string) (*datastore.Key, error) {
	comps := strings.Split(name, "/")
	if comps[0] != "refs" {
		return nil, repository.ErrInvalidRef
	}
	key := r.refs
	for _, comp := range comps[1:] {
		if comp == "" {
			return nil, repository.ErrInvalidRef
		}
		key = datastore.NewKey(r.ctx, "ref", comp, 0, key)
	}
	return key, nil
}

func (r *repo) GetRef(name string) (object.ID, error) {
	var id object.ID
	key, err := r.refKey(name)
	if err != nil {
		return id, err
	}
	return id, r.get(key, &id)
}

func (r *repo) SetRef(name string, id object.ID) error {
	key, err := r.refKey(name)
	if err != nil {
		return err
	}
	return r.put(key, &id)
}

func (r *repo) DelRef(name string) error {
	key, err := r.refKey(name)
	if err != nil {
		return err
	}
	return r.del(key)
}

func (r *repo) ListRefs(prefix string) ([]string, []object.ID, error) {
	key, err := r.refKey(prefix)
	if err != nil {
		return nil, nil, err
	}
	var ids []object.ID
	keys, err := datastore.NewQuery("ref").
		Ancestor(key).
		Order("__key__").
		GetAll(r.ctx, &ids)
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, len(keys))
	for i, key := range keys {
		for !r.refs.Equal(key) {
			names[i] = "/" + key.StringID() + names[i]
			key = key.Parent()
		}
		names[i] = "refs" + names[i]
	}
	return names, ids, nil
}

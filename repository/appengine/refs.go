package appengine

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/repository"
)

func mapRefErr(err error) error {
	if err == datastore.ErrNoSuchEntity {
		return repository.ErrRefNotExist
	}
	return err
}

func (r *repo) refKey(name string) (*datastore.Key, error) {
	if !repository.IsValidRef(name) {
		return nil, repository.ErrInvalidRef
	}
	return datastore.NewKey(r.ctx, r.prefix+"ref", name, 0, r.root), nil
}

func (r *repo) GetRef(name string) (object.ID, error) {
	var id object.ID
	key, err := r.refKey(name)
	if err != nil {
		return id, err
	}
	err = mapRefErr(r.get(key, &id))
	return id, err
}

func (r *repo) UpdateRef(name string, oldID, newID object.ID) error {
	key, _ := r.refKey(name)
	return datastore.RunInTransaction(r.ctx, func(tc context.Context) error {
		var id object.ID
		tr := *r
		tr.ctx = tc
		if err := tr.get(key, &id); err != nil && err != datastore.ErrNoSuchEntity {
			return err
		}
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
			if oldID == object.ZeroID {
				return nil
			}
			return mapRefErr(tr.del(key))
		default:
			if _, err := tr.GetObject(newID); err != nil {
				return err
			}
			return tr.put(key, &newID)
		}
	}, &datastore.TransactionOptions{Attempts: 0})
}

func (r *repo) ListRefs() ([]string, []object.ID, error) {
	var ids []object.ID
	keys, err := datastore.NewQuery(r.prefix+"ref").
		Ancestor(r.root).
		Order("__key__").
		GetAll(r.ctx, &ids)
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = key.StringID()
	}
	return names, ids, nil
}

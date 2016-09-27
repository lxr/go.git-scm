package appengine

import (
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/repository"
)

func (r *repo) GetObject(id object.ID) (object.Interface, error) {
	if obj, err := r.getObjectMemcache(id); err != memcache.ErrCacheMiss {
		return obj, err
	}
	obj, err := r.getObject(id)
	if err == nil {
		r.putObjectMemcache(id, obj)
	}
	return obj, err
}

func (r *repo) PutObject(obj object.Interface) (object.ID, error) {
	id, err := r.putObject(obj)
	if err == nil {
		r.putObjectMemcache(id, obj)
	}
	return id, err
}

func (r *repo) objKey(objType object.Type, id object.ID) *datastore.Key {
	return datastore.NewKey(r.ctx, r.prefix+objType.String(), id.String(), 0, nil)
}

func (r *repo) getObject(id object.ID) (object.Interface, error) {
	for t := object.TypeCommit; t < object.TypeReserved; t++ {
		obj, _ := object.New(t)
		err := datastore.Get(r.ctx, r.objKey(t, id), obj)
		switch err {
		case nil:
			return obj, nil
		case datastore.ErrNoSuchEntity:
			// try the next object type
		default:
			return nil, err
		}
	}
	return nil, repository.ErrNotExist
}

func (r *repo) putObject(obj object.Interface) (object.ID, error) {
	id, err := object.Hash(obj)
	if err == nil {
		t := object.TypeOf(obj)
		_, err = datastore.Put(r.ctx, r.objKey(t, id), obj)
	}
	return id, err
}

func (r *repo) objKeyMemcache(id object.ID) string {
	return r.prefix + id.String()
}

func (r *repo) getObjectMemcache(id object.ID) (object.Interface, error) {
	item, err := memcache.Get(r.ctx, r.objKeyMemcache(id))
	if err != nil {
		return nil, err
	}
	return object.Unmarshal(item.Value)
}

func (r *repo) putObjectMemcache(id object.ID, obj object.Interface) error {
	data, err := object.Marshal(obj)
	if err != nil {
		return err
	}
	return memcache.Set(r.ctx, &memcache.Item{
		Key:   r.objKeyMemcache(id),
		Value: data,
	})
}

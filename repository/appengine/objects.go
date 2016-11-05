package appengine

import (
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/repository"
)

func (r *repo) GetObject(id object.ID) (object.Interface, error) {
	if data, err := r.getObjectMemcache(id); err == nil {
		return object.Unmarshal(data)
	} else if err != memcache.ErrCacheMiss {
		return nil, err
	}
	obj, err := r.getObject(id)
	if err == nil {
		data, err := obj.MarshalBinary()
		if err == nil {
			r.putObjectMemcache(id, data)
		}
	}
	return obj, err
}

func (r *repo) PutObject(obj object.Interface) (object.ID, error) {
	id, data, err := r.putObject(obj)
	if err == nil {
		r.putObjectMemcache(id, data)
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
	return nil, repository.ErrObjectNotExist
}

func (r *repo) putObject(obj object.Interface) (object.ID, []byte, error) {
	data, id, err := object.Marshal(obj)
	if err == nil {
		t := object.TypeOf(obj)
		_, err = datastore.Put(r.ctx, r.objKey(t, id), obj)
	}
	return id, data, err
}

func (r *repo) objKeyMemcache(id object.ID) string {
	return r.prefix + id.String()
}

func (r *repo) getObjectMemcache(id object.ID) ([]byte, error) {
	item, err := memcache.Get(r.ctx, r.objKeyMemcache(id))
	if err != nil {
		return nil, err
	}
	return item.Value, nil
}

func (r *repo) putObjectMemcache(id object.ID, data []byte) error {
	return memcache.Set(r.ctx, &memcache.Item{
		Key:   r.objKeyMemcache(id),
		Value: data,
	})
}

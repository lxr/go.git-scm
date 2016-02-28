// Package appengine implements a Git repository backed by the Google
// App Engine datastore and memcache.
package appengine

// TODO(lor): Document how Git objects are stored in the datastore.

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/memcache"

	"github.com/lxr/go.git-scm/repository"
)

func mapErr(err error) error {
	if err == datastore.ErrNoSuchEntity {
		return repository.ErrNotExist
	}
	return err
}

// OpenRepository returns a Git repository interface to the App Engine
// datastore using the given context and rooted at the given key.
// The prefix string is used to "namespace" any memcache accesses by
// prepending it to all memcache keys.
func OpenRepository(ctx context.Context, root *datastore.Key, prefix string) repository.Interface {
	return &repo{
		ctx:     ctx,
		objects: datastore.NewKey(ctx, "category", "objects", 0, root),
		refs:    datastore.NewKey(ctx, "category", "refs", 0, root),
		head:    datastore.NewKey(ctx, "head", "HEAD", 0, root),
		prefix:  prefix,
	}
}

type repo struct {
	ctx     context.Context
	objects *datastore.Key
	refs    *datastore.Key
	head    *datastore.Key
	prefix  string
}

func (r *repo) Init() error {
	return r.SetHead("", "refs/heads/master")
}

func (r *repo) memkey(key *datastore.Key) string {
	return r.prefix + key.Encode()
}

func (r *repo) get(key *datastore.Key, dst interface{}) error {
	s := r.memkey(key)
	if _, err := memcache.Gob.Get(r.ctx, s, dst); err != memcache.ErrCacheMiss {
		return err
	}
	if err := mapErr(datastore.Get(r.ctx, key, dst)); err != nil {
		return err
	}
	memcache.Gob.Set(r.ctx, &memcache.Item{Key: s, Object: dst})
	return nil
}

func (r *repo) put(key *datastore.Key, src interface{}) error {
	if _, err := datastore.Put(r.ctx, key, src); err != nil {
		return err
	}
	s := r.memkey(key)
	memcache.Gob.Set(r.ctx, &memcache.Item{Key: s, Object: src})
	return nil
}

func (r *repo) del(key *datastore.Key) error {
	memcache.Delete(r.ctx, r.memkey(key))
	return mapErr(datastore.Delete(r.ctx, key))
}

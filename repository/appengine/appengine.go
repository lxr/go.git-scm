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

// InitRepository initializes a new Git repository structure in the App
// Engine datastore using the given context and rooted at the given key.
// The prefix string is used to "namespace" entity kinds and memcache
// accesses by prepending it to kind names and memcache keys.
func InitRepository(ctx context.Context, root *datastore.Key, prefix string) (repository.Interface, error) {
	r := &repo{
		ctx:    ctx,
		root:   root,
		prefix: prefix,
	}
	return r, r.SetHEAD("refs/heads/master")
}

// OpenRepository returns a Git repository interface to the App Engine
// datastore using the given context and rooted at the given key.  The
// prefix string is used to "namespace" entity kinds and memcache
// accesses by prepending it to kind names and memcache keys.
func OpenRepository(ctx context.Context, root *datastore.Key, prefix string) repository.Interface {
	return &repo{
		ctx:    ctx,
		root:   root,
		prefix: prefix,
	}
}

type repo struct {
	ctx    context.Context
	root   *datastore.Key
	prefix string
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

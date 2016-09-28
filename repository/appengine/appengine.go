// Package appengine implements a Git repository backed by the Google
// App Engine datastore and memcache.  See the documentation for the
// InitRepository function for how the repository is stored in the
// datastore.
package appengine

// BUG(lor): The datastore limits
// (https://cloud.google.com/datastore/docs/concepts/limits) apply to
// package appengine too, which restricts its usefulness as a large-file
// repository backend.

// BUG(lor): The datastore schema does not distinguish objects between
// repositories; all objects effectively belong to the same giant object
// store.  Only reachability from refs denotes "membership" in a
// repository.

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

// InitRepository initializes a new Git repository in the App Engine
// datastore using the given context.  A repository is stored in the
// datastore using six kinds of entities: HEADs, refs, commits, trees,
// blobs and tags, with the following schema:
//
// 	// actual kind depends on the kind of the root key:
// 		HEAD (string)
//
// 	<prefix>ref:
// 		ID (string)
//
// 	<prefix>commit:
// 		Tree (string) // object ID
// 		Parent (list(string)) // object IDs
// 		Author.Date (datetime)
// 		Author.Email (string)
// 		Author.Name (string)
// 		Author.TZ (int) // offset from GMT in seconds
// 		Committer.Date (datetime)
// 		Committer.Email (string)
// 		Committer.Name (string)
// 		Committer.TZ (int) // offset from GMT in seconds
// 		Message (Text) // not indexed
//
// 	<prefix>tree:
// 		Name (list(string))
// 		Mode (list(int))
// 		Object (list(string)) // object IDs
//
// 	<prefix>blob:
// 		Contents (Blob) // not indexed
//
// 	<prefix>tag:
// 		Object (string) // object ID
// 		Type (string)
// 		Tag (string)
// 		Tagger.Date (datetime)
// 		Tagger.Email (string)
// 		Tagger.Name (string)
// 		Tagger.TZ (int) // offset from GMT in seconds
// 		Message (Text) // not indexed
//
// The prefix string is prepended to each kind name in order to provide
// a means of avoiding kind name collisions with other applications
// using the datastore.  It is similarly prepended to each memcache
// key to avoid memcache collisions.
//
// In addition to storing the repository HEAD, the root key is used as
// the parent of all ref keys, whose names are the names of the refs.
//
// For performance, all objects are stored without a parent key (i.e.
// without an entity group).  The name of an object's key is the
// hexadecimal representation of its ID.
//
// InitRepository does not clear already initialized repos; it merely
// sets the HEAD to point to refs/heads/master.
func InitRepository(ctx context.Context, root *datastore.Key, prefix string) (repository.Interface, error) {
	r := &repo{
		ctx:    ctx,
		root:   root,
		prefix: prefix,
	}
	return r, r.SetHEAD("refs/heads/master")
}

// OpenRepository returns a Git repository interface to the App Engine
// datastore using the given parameters.  Refer to the documentation for
// the InitRepository function to see how they control access to the
// repository.
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

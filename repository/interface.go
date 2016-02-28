// Package repository defines a common interface for Git repositories.
// It also defines a number of convenience functions for querying and
// manipulating types that implement the interface.
package repository

import (
	"errors"

	"github.com/lxr/go.git-scm/object"
)

// ErrNotExist is returned when the named object, ref or head does not
// exist.
var ErrNotExist = errors.New("repository: item does not exist")

// ErrInvalidRef is returned when a refname parameter is not
// well-formed.
var ErrInvalidRef = errors.New("repository: invalid refname")

// Interface defines the interface to a Git repository.  A Git
// repository is a database storing three types of items:
//
//  - Git objects (commits, blobs, trees and tags),
//  - refs (references to Git objects, usually commits or tags), and
//  - heads (references to "active" refs, which must be dereferencable
//    to commits; more commonly known as symbolic refs).
//
// A Git object is identified by its ID, a ref by its name, and a head
// by the name of the remote repository it is the HEAD of.  The head of
// the local repository is identified by the empty string.
//
// The interface defines separate sets of getter/setter methods for each
// of these types.  The Get and Del methods return ErrNotExist if
// the item does not exist.
type Interface interface {
	// Init performs whatever backend-specific work there is to
	// make a newly created repository usable.  Usually this
	// involves at least pointing the repository head to
	// refs/heads/master.
	Init() error
	// TODO(lor): The Interface.Init method rather confuses the
	// interface.  Implementors should be required to provide a
	// CreateRepository method or the like.

	// The object methods.  PutObject may return err = nil in the
	// (vanishingly unlikely) case that a different object with the
	// same ID already exists.
	GetObject(id object.ID) (object.Interface, error)
	PutObject(obj object.Interface) (object.ID, error)
	DelObject(id object.ID) error

	// The ref methods.  It is the user's responsibility to ensure
	// that the refnames passed to these methods are well-formed,
	// though implementations must return ErrInvalidRef rather than
	// panic on an ill-formed ref.  These methods do not resolve
	// symbolic refs.
	GetRef(name string) (object.ID, error)
	SetRef(name string, id object.ID) error
	DelRef(name string) error

	// ListRefs lists all refs in the repository whose names begin
	// with the given prefix in ascending order by C locale.  The
	// prefix is a sequence of slash-separated path components;
	// it must not have a leading or trailing slash.  The listing
	// does not include symbolic refs, in particular the symbolic
	// ref HEAD.
	ListRefs(prefix string) ([]string, []object.ID, error)

	// BUG(lor): The definition of Interface.ListRefs makes a
	// bit-perfect reimplementation of the "git show-ref" command
	// troublesome, as it includes remote heads, which ARE symbolic
	// refs.  I believe this is unintended behavior, however, since
	// symrefs were never intended to be a similar "class" of
	// entities to normal refs (inferred from
	// http://permalink.gmane.org/gmane.comp.version-control.git/166812).

	// The head methods.
	GetHead(name string) (string, error)
	SetHead(name string, target string) error
	DelHead(name string) error
}

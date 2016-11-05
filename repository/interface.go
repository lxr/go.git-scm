// Package repository defines a common interface for Git repositories
// and a set of convenience functions for manipulating them.
package repository

import (
	"errors"

	"github.com/lxr/go.git-scm/object"
)

// Repository error conditions.
var (
	ErrInvalidRef     = errors.New("repository: malformed refname")
	ErrRefMismatch    = errors.New("repository: ref value is different from expected")
	ErrRefExist       = errors.New("repository: ref already exists")
	ErrRefNotExist    = errors.New("repository: ref does not exist")
	ErrObjectNotExist = errors.New("repository: object does not exist")
)

// Interface defines the interface of a Git repository.  A Git
// repository is a database storing three types of objects:
//
//  - Git objects (commits, blobs, trees and tags), which form an
//    immutable graph structure through embedded links,
//  - refs, which represent entry points to this graph, and
//  - HEAD, a special singleton pointing to the "current" ref.
//
// Git objects are identified by their IDs and refs by their names.
//
// Implementations must provide safe concurrent access to the
// underlying datastore, even between different instances with the same
// backing store.
type Interface interface {
	// BUG(lor): Pseudo- and symbolic refs, commit hooks etc. are
	// beyond the scope of Interface.  It supports only the bare
	// minimum functionality necessary for exchanging repository
	// data.

	// GetObject returns the object with the given ID.
	GetObject(id object.ID) (object.Interface, error)

	// PutObject stores the given object in the repository and
	// calculates and returns its ID.  Storing the same object
	// multiple times is idempotent.  Behavior is undefined if a
	// different object that hashes to the same ID is stored;
	// implementations may document their own behavior.
	PutObject(obj object.Interface) (object.ID, error)

	// GetRef returns the ID of the object the named ref points to.
	GetRef(name string) (object.ID, error)

	// UpdateRef atomically changes the named ref to point from
	// oldID to newID.  It is an error if either the ref does
	// not point at oldID at the time of the call, or the object
	// named by newID does not exist in the repository.  The
	// function is special-cased when one or both of oldID and newID
	// is zero:
	//
	//  - if oldID is zero, the ref is created if it does not
	//    exist, and ErrRefExist is returned if it does;
	//  - if newID is zero, the ref is deleted if it exists;
	//  - if both newID and oldID are zero, UpdateRef returns with
	//    a nil error only if the named ref does not exist in the
	//    repository.
	UpdateRef(name string, oldID, newID object.ID) error

	// ListRefs lists all refs in the repository in ascending order
	// by C locale.
	ListRefs() ([]string, []object.ID, error)

	// GetHEAD returns the name of the ref the HEAD points to.
	GetHEAD() (string, error)

	// SetHEAD sets HEAD to point to the named ref.
	SetHEAD(name string) error
}

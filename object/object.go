// Package object implements the Git object model.
package object

import (
	"crypto/sha1"
	"encoding"
	"encoding/hex"
	"errors"
	"fmt"
)

var errBadIDLen = errors.New("object: invalid ID length")

// Interface defines the functionality expected of a Git object.
//
// A Git object has a canonical binary representation (see
// http://git.rsbx.net/Documents/Git_Data_Formats.txt), whose SHA-1
// digest is the object's name.  The methods MarshalBinary and
// UnmarshalBinary encode and decode Git objects to and from these
// representations.  An object additionally has a human-readable
// representation (returned by the reference Git client's "cat-file -p"
// command), which is encoded and decoded with MarshalText and
// UnmarshalText.  For all objects except Tree, the binary
// representation is just the textual representation prefixed with the
// Git object header.
//
// Though it is possible for an external type to satisfy this interface,
// functions operating on it should not be expected to work with
// implementations other than the ones defined in this package.
// It is exported only for convenience of documentation and development.
type Interface interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	encoding.TextMarshaler
	encoding.TextUnmarshaler
}

// NOTE(lor): The (Un)marshalBinary methods include the Git object
// header in their in/output for type checking purposes.  This results
// in some duplicated code, but otherwise all byte slices would
// unmarshal successfully into a Blob, which breaks the semantics of
// e.g. encoding/gob.

// BUG(lor): The (Un)marshal* methods of the Git objects perform no
// input sanitization, so it is possible to unmarshal objects that the
// standard Git implementation would never create, and even to create
// objects that cannot be unmarshaled once marshaled.  Use care and
// common sense when manipulating the objects.

// BUG(lor): Interface probably shouldn't embed TextMarshaler and
// -Unmarshaler, since the objects' textual representations aren't
// canonical by any real measure, which the two methods imply.  For
// instance, json.Marshal tries MarshalText before using reflection,
// which probably isn't what you want if you want to serialize Git
// objects in JSON.

// An ID is the name of a Git object.
type ID [sha1.Size]byte

// ZeroID (20 zero bytes) is used to designate a nonexistent object.
var ZeroID ID

// Hash computes the ID of a Git object.  It returns a TypeError
// containing obj if it is not one of the standard Git objects.
func Hash(obj Interface) (ID, error) {
	data, err := Marshal(obj)
	if err != nil {
		return ZeroID, err
	}
	return ID(sha1.Sum(data)), nil
}

// DecodeID parses a 40-character hexadecimal string as a Git ID.
func DecodeID(s string) (id ID, err error) {
	b, err := hex.DecodeString(s)
	switch {
	case err != nil:
		return id, err
	case len(b) != len(id):
		return id, errBadIDLen
	}
	copy(id[:], b)
	return id, err
}

// String returns the ID as a lowercase 40-digit hexadecimal string.
func (id ID) String() string {
	return hex.EncodeToString(id[:])
}

// Scan is a support routine for fmt.Scanner.  The format verb is
// ignored; Scan always attempts to read 40 hexadecimal digits from
// the input.
func (id *ID) Scan(ss fmt.ScanState, verb rune) error {
	var p []byte
	if _, err := fmt.Fscanf(ss, "%40x", &p); err != nil {
		return err
	}
	if copy((*id)[:], p) != len(*id) {
		return errBadIDLen
	}
	return nil
}

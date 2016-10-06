package object

import (
	"fmt"
	"io"
)

// Type enumerates the standard Git object types.
type Type byte

const (
	TypeUnknown Type = iota

	TypeCommit
	TypeTree
	TypeBlob
	TypeTag

	TypeReserved
)

// A TypeError is used to report an invalid or unknown Git object type.
// Methods returning a TypeError specify the concrete type of the value
// it holds.
type TypeError struct {
	Value interface{}
}

func (e *TypeError) Error() string {
	if t, ok := e.Value.(Type); ok {
		return fmt.Sprintf("bad Git type code: %#x", t)
	} else {
		return fmt.Sprintf("bad Git object type: %v", e.Value)
	}
}

// TypeOf returns the type of the given object, or TypeUnknown if it is
// not one of the standard Git object types.
func TypeOf(obj Interface) Type {
	switch obj.(type) {
	case *Commit:
		return TypeCommit
	case *Tree:
		return TypeTree
	case *Blob:
		return TypeBlob
	case *Tag:
		return TypeTag
	default:
		return TypeUnknown
	}
}

// String returns "commit", "tree", "blob" or "tag" depending on the
// value of the type.  It returns an empty string if the type is not one
// of the standard Git ones.
func (t Type) String() string {
	switch t {
	case TypeCommit:
		return "commit"
	case TypeTree:
		return "tree"
	case TypeBlob:
		return "blob"
	case TypeTag:
		return "tag"
	default:
		return ""
	}
}

// Scan is a support routine for fmt.Scanner.  It reads a
// whitespace-delimited word from input and attempts to interpret it
// as one of the strings returned by String.  If the word is not
// recognized, a TypeError containing it is returned.
func (t *Type) Scan(ss fmt.ScanState, verb rune) error {
	tok, err := ss.Token(true, nil)
	switch {
	case err != nil:
		return err
	case len(tok) == 0:
		return io.ErrUnexpectedEOF
	}
	switch string(tok) {
	case "commit":
		*t = TypeCommit
	case "tree":
		*t = TypeTree
	case "blob":
		*t = TypeBlob
	case "tag":
		*t = TypeTag
	default:
		return &TypeError{string(tok)}
	}
	return nil
}

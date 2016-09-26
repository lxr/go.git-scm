package object

import (
	"bytes"
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

// New returns a pointer to a newly allocated zero value of a Git object
// of the given type.  It returns a TypeError containing the objType
// argument if it is not one of the standard Git object types.  New
// never returns an error otherwise.
func New(objType Type) (Interface, error) {
	switch objType {
	case TypeCommit:
		return new(Commit), nil
	case TypeTree:
		return new(Tree), nil
	case TypeBlob:
		return new(Blob), nil
	case TypeTag:
		return new(Tag), nil
	default:
		return nil, &TypeError{objType}
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

// Marshal returns the canonical binary representation of the given
// object.  It returns a TypeError containing obj if it is not one of
// the standard Git objects.
func Marshal(obj Interface) ([]byte, error) {
	if TypeOf(obj) == TypeUnknown {
		return nil, &TypeError{obj}
	}
	return obj.MarshalBinary()
}

// Unmarshal decodes a Git object from its canonical binary
// representation.  If the type recorded in the Git object header does
// not match one of the standard Git ones, it is returned as a string
// inside a TypeError.
func Unmarshal(data []byte) (Interface, error) {
	r := bytes.NewReader(data)
	var objType Type
	var length int
	if _, err := fmt.Fscanf(r, "%s %d\x00", &objType, &length); err != nil {
		return nil, err
	}
	obj, _ := New(objType)
	return obj, obj.UnmarshalBinary(data)
}

// prependHeader prepends a Git object header to an object's binary
// representation.  It returns a TypeError containing the objType
// argument if it is not one of the standard Git ones.
func prependHeader(objType Type, data []byte) ([]byte, error) {
	if objType.String() == "" {
		return nil, &TypeError{objType}
	}
	header := []byte(fmt.Sprintf("%s %d\x00", objType, len(data)))
	return append(header, data...), nil
}

// stripHeader strips the Git object header from an object's binary
// representation and validates the type and length recorded in it.
// It returns the remaining data in the representation.  It returns a
// TypeError containing the objType argument if it is not one of the
// standard Git ones.
func stripHeader(objType Type, data []byte) ([]byte, error) {
	if objType.String() == "" {
		return nil, &TypeError{objType}
	}
	buf := bytes.NewBuffer(data)
	var bufType Type
	var length int
	_, err := fmt.Fscanf(buf, "%s %d\x00", &bufType, &length)
	switch {
	case err != nil:
		return nil, err
	case bufType != objType:
		return nil, fmt.Errorf("object: expected type %s, got %s", objType, bufType)
	case length != buf.Len():
		return nil, fmt.Errorf("object: expected length %d, got %d", length, buf.Len())
	default:
		return buf.Bytes(), err
	}
}

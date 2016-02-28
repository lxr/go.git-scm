// Package internal defines certain functions package packfile and its
// subpackages need.
package internal

import (
	"bytes"
	"fmt"

	"github.com/lxr/go.git-scm/object"
)

// MarshalObj returns the binary representation of a Git object minus
// the object header.  It returns an *object.TypeError containing the
// obj argument if it is not one of the standard Git objects.
func MarshalObj(obj object.Interface) ([]byte, error) {
	data, err := object.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return data[bytes.IndexByte(data, 0)+1:], nil
}

// UnmarshalObj decodes a Git object from its binary representation
// minus the object header.  It returns an *object.TypeError containing
// the obj argument if it is not one of the standard Git objects.
func UnmarshalObj(obj object.Interface, data []byte) error {
	objType := object.TypeOf(obj)
	if objType == object.TypeUnknown {
		return &object.TypeError{obj}
	}
	header := []byte(fmt.Sprintf("%s %d\x00", objType, len(data)))
	return obj.UnmarshalBinary(append(header, data...))
}

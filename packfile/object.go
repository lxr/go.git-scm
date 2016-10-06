// Git objects are stored in packfiles with a special type of header.
// The functions in this file read and write these headers and marshal
// and unmarshal objects to and from headerless representations.  The
// reason the header processing is separate from the marshaling -
// unlike in object.Interface - is because the bodies of Git objects
// are zlibbed in packfiles, while their headers are not.

package packfile

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"

	"github.com/lxr/go.git-scm/object"
)

// A packfile object header is a little-endian base128-encoded number
// where bits 4-6 encode the object's type and the rest its size.

func readObjHeader(r io.ByteReader) (object.Type, int64, error) {
	hdr, err := readBase128LE(r)
	if err != nil {
		return 0, 0, err
	}
	objType := object.Type(hdr >> 4 & 0x7)
	size := int64((hdr >> 3 &^ 0xF) | (hdr & 0xF))
	return objType, size, err
}

func writeObjHeader(w io.Writer, objType object.Type, size int64) error {
	// XXX(lor): Objects larger than 2305843009213693951 bytes
	// (0x1FFFFFFFFFFFFFFF in hex) cannot be read or written, as an
	// object's size is internally represented as a 64-bit integer,
	// of which three bits are reserved for encoding the object's
	// type.
	if size < 0 || size > 0x1FFFFFFFFFFFFFFF {
		return errors.New("packfile: object size out of range")
	}
	hdr := uint64((size &^ 0xF << 3) | int64(objType<<4) | (size & 0xF))
	_, err := writeBase128LE(w, hdr)
	return err
}

func marshalObj(obj object.Interface) ([]byte, error) {
	data, _, err := object.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return data[bytes.IndexByte(data, 0)+1:], nil
}

func unmarshalObj(obj object.Interface, data []byte) error {
	objType := object.TypeOf(obj)
	if objType == object.TypeUnknown {
		return &object.TypeError{obj}
	}
	header := []byte(fmt.Sprintf("%s %d\x00", objType, len(data)))
	return obj.UnmarshalBinary(append(header, data...))
}

func hashObj(objType object.Type, data []byte) object.ID {
	header := []byte(fmt.Sprintf("%s %d\x00", objType, len(data)))
	return object.ID(sha1.Sum(append(header, data...)))
}

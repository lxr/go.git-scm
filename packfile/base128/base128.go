// Package base128 implements the Git packfile variable-length number
// encoding.  The encoding uses the standard "MSB set = more bytes
// follow" scheme, but both little- and big-endian encodings are used,
// and the big-endian encoding comes with a twist.  The little-endian
// encoding is exactly the same as the one used by encoding/binary
// (see https://developers.google.com/protocol-buffers/docs/encoding#varints).
// The big-endian encoding is called "modified big-endian" and involves
// adding/substracting one to/from the number before/after shifting it
// during decoding/encoding.  Refer to http://git.rsbx.net/Documents/Git_Data_Formats.txt
// and the source code for this package for clarification.
package base128

import (
	"encoding/binary"
	"errors"
	"io"
)

// ReadLE reads a little-endian base128-encoded number from r.
// It returns an error if the encoded number does not fit in 64 bits.
func ReadLE(r io.ByteReader) (uint64, error) {
	return binary.ReadUvarint(r)
}

// WriteLE writes a little-endian base128-encoded number to w and
// returns the number of bytes written.
func WriteLE(w io.Writer, x uint64) (int, error) {
	var p [10]byte
	n := binary.PutUvarint(p[:], x)
	return w.Write(p[:n])
}

// ReadMBE reads a modified big-endian base128-encoded number from r.
// It returns an error if the encoded number does not fit in 64 bits.
func ReadMBE(r io.ByteReader) (uint64, error) {
	c, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	x := uint64(c & 0x7F)
	for c&0x80 != 0 {
		if x >= 1<<57-1 {
			return x, errors.New("base128: MBE-encoded number overflows a 64-bit integer")
		}
		c, err = r.ReadByte()
		if err != nil {
			return x, err
		}
		x = (x+1)<<7 | uint64(c&0x7F)
	}
	return x, nil
}

// WriteMBE writes a modified big-endian base128-encoded number to w
// and returns the number of bytes written.
func WriteMBE(w io.Writer, x uint64) (int, error) {
	var p [10]byte
	i := len(p) - 1
	p[i] = byte(x) & 0x7F
	x = x>>7 - 1
	for x != ^uint64(0) {
		i--
		p[i] = byte(x) | 0x80
		x = x>>7 - 1
	}
	return w.Write(p[i:])
}

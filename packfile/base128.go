// The functions in this file implement the Git packfile variable-length
// number encoding.  The encoding uses the standard "MSB set = more
// bytes follow" scheme, but both little- and big-endian encodings are
// used, and the big-endian encoding comes with a twist.  The
// little-endian encoding is exactly the same as the one used by
// encoding/binary (see https://developers.google.com/protocol-buffers/docs/encoding#varints).
// The big-endian encoding is called "modified big-endian" and involves
// adding/substracting one to/from the number before/after shifting it
// during decoding/encoding.  Refer to http://git.rsbx.net/Documents/Git_Data_Formats.txt
// and this source for clarification.

package packfile

import (
	"encoding/binary"
	"errors"
	"io"
)

// base128LE decodes a uint64 from buf and returns that value and the
// number of bytes read (> 0).  If an error occurred, the value is 0 and
// the number of bytes n is <= 0 meaning:
//
//	n == 0: buf too small
//	n  < 0: value larger than 64 bits (overflow)
//	     and -n is the number of bytes read
func base128LE(buf []byte) (uint64, int) {
	return binary.Uvarint(buf)
}

// putBase128LE encodes a uint64 into buf and returns the number of
// bytes written.  If the buffer is too small, putBase128LE will panic.
func putBase128LE(buf []byte, x uint64) int {
	return binary.PutUvarint(buf, x)
}

// readBase128LE reads a little-endian base128-encoded number from r.
// It returns an error if the encoded number does not fit in 64 bits.
func readBase128LE(r io.ByteReader) (uint64, error) {
	return binary.ReadUvarint(r)
}

// writeBase128LE writes a little-endian base128-encoded number to w
// and returns the number of bytes written.
func writeBase128LE(w io.Writer, x uint64) (int, error) {
	var p [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(p[:], x)
	return w.Write(p[:n])
}

// readBase128MBE reads a modified big-endian base128-encoded number
// from r.  It returns an error if the encoded number does not fit in
// 64 bits.
func readBase128MBE(r io.ByteReader) (uint64, error) {
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

// writeBase128MBE writes a modified big-endian base128-encoded number
// to w and returns the number of bytes written.
func writeBase128MBE(w io.Writer, x uint64) (int, error) {
	var p [binary.MaxVarintLen64]byte
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

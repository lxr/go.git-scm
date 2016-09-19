// To save space, certain objects in Git packfiles are stored as deltas:
// differences from an earlier object in the stream.  The functions in
// this file implement resolving such deltas.  For details on their
// binary representation, see http://git.rsbx.net/Documents/Git_Data_Formats.txt.

package packfile

import (
	"errors"

	"github.com/lxr/go.git-scm/object"
)

// These errors can be returned during delta resolution.
var (
	// ErrDelta is returned when applying a delta object fails
	// sanity checks.
	ErrDelta = errors.New("packfile: delta does not apply cleanly")
	// ErrDeltaLength is returned if an invalid length is encoded
	// in a delta object body.
	ErrDeltaLength = errors.New("packfile: invalid length in delta object")
)

// The "type" of a delta object defines how its base object is
// referenced: with its byte offset from the current position within
// the packfile stream, or by its object ID.
const (
	offsetDelta object.Type = 6
	refDelta    object.Type = 7
)

func applyDelta(base, delta []byte) (result []byte, err error) {
	defer func() {
		if e, ok := recover().(error); ok {
			err = e
		}
	}()

	var i, j int
	baseLen, n := base128LE(delta[i:])
	if n <= 0 {
		return nil, ErrDeltaLength
	}
	i += n
	if baseLen != uint64(len(base)) {
		return nil, ErrDelta
	}
	resultLen, n := base128LE(delta[i:])
	if n <= 0 {
		return nil, ErrDeltaLength
	}
	i += n
	result = make([]byte, resultLen)
	for i < len(delta) {
		opcode := delta[i]
		i += 1
		switch opcode >> 7 {
		case 0: // insert
			n := int(opcode)
			j += copy(result[j:], delta[i:i+n])
			i += n
		case 1: // copy
			off, n := uvarintMask(delta[i:], (opcode & 0x0F))
			if n < 0 {
				return nil, ErrDeltaLength
			}
			i += n
			len, n := uvarintMask(delta[i:], (opcode&0x70)>>4)
			if n < 0 {
				return nil, ErrDeltaLength
			}
			i += n
			if len == 0 {
				len = 1 << 16
			}
			j += copy(result[j:], base[off:off+len])
		default:
			panic("byte has more than 8 bits")
		}
	}
	if resultLen != uint64(j) {
		return nil, ErrDelta
	}
	return result, nil
}

// uvarintMask and putUvarintMask read and write "bitmask-compressed"
// unsigned integers.  A bitmask-compressed integer is encoded as a
// little-endian integer with all zero bytes omitted; a separate 8-bit
// mask communicates which bytes are present, with less significant bits
// corresponding to less significant bytes.  A byte is present if and
// only if its bit is set in the mask.

// uvarintMask decodes a uint64 from buf using mask and returns that
// value and the number of bytes read (>=0).  If an error occurred, the
// value is 0 and the number of bytes n is <0, meaning that buf is too
// small.
func uvarintMask(buf []byte, mask uint8) (x uint64, n int) {
	for i := uint(0); i < 8; i++ {
		if mask&(1<<i) != 0 {
			if n >= len(buf) {
				return 0, -1
			}
			x |= uint64(buf[n]) << (i * 8)
			n++
		}
	}
	return
}

// putUvarintMask encodes a uint64 into buf and returns its bitmask and
// the number of bytes written.  It panics if the number does not fit
// into buf.
func putUvarintMask(buf []byte, x uint64) (mask uint8, n int) {
	for i := uint(0); i < 8; i++ {
		c := byte(x >> (i * 8))
		if c != 0 {
			buf[n] = c
			n++
			mask |= 1 << i
		}
	}
	return
}

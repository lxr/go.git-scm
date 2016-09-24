// To save space, certain objects in Git packfiles are stored as deltas:
// differences from an earlier object in the stream.  The functions in
// this file implement resolving and calculating such deltas.  For
// details on their binary representation, see http://git.rsbx.net/Documents/Git_Data_Formats.txt.

package packfile

import (
	"encoding/binary"
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

// Delta instruction parameter limits.  computeDelta is responsible for
// not generating delta instructions that exceed these values.
//
// maxCopyOff could technically be 0xFFFFFFFF, as the copy offset is
// stored as an unsigned 32-bit integer, but computeDelta uses a regular
// int in order to avoid conversion noise in its arithmetic, so we do
// not reserve bit 31 in case ints are 32-bit on this machine.
const (
	maxCopyOff   = 0x7FFFFFFF
	maxCopyLen   = 0xFFFFFF
	maxInsertLen = 0x7F
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

func computeDelta(result, base []byte) (delta []byte) {
	var n, baseOff int
	var buf [2 * binary.MaxVarintLen64]byte
	n += putBase128LE(buf[n:], uint64(len(base)))
	n += putBase128LE(buf[n:], uint64(len(result)))
	delta = make([]byte, n)
	copy(delta, buf[:])
	for len(result) >= maxInsertLen && len(base) >= maxInsertLen &&
		baseOff < maxCopyOff-maxInsertLen {
		i, j, n := longestCommonSubstring(result[:maxInsertLen], base[:maxInsertLen])
		// Try to extend the match if it happens to end at the
		// base slice boundary.
		if j+n == maxInsertLen {
			for i+n < len(result) && j+n < len(base) &&
				n < maxCopyLen && result[i+n] == base[j+n] {
				n++
			}
		}
		// Using copy instructions for slices of 6 bytes or less
		// is generally not worth it.  Have the whole interval
		// inserted instead.
		if n <= 6 {
			i = maxInsertLen
			j = maxInsertLen
			n = 0
		}
		if i > 0 {
			delta = append(append(delta, byte(i)), result[:i]...)
		}
		if n > 0 {
			offmask, offn := putUvarintMask(buf[0:], uint64(baseOff+j))
			lenmask, lenn := putUvarintMask(buf[offn:], uint64(n))
			delta = append(delta, 0x80|(lenmask<<4)|offmask)
			delta = append(delta, buf[:offn+lenn]...)
		}
		// When i+n and j+n are less than maxInsertLen, some
		// bytes will be involved in multiple
		// longestCommonSubstring searches.  However, as both
		// search windows move at least 6 bytes to the right
		// every iteration, each byte will be involved in only
		// a constant number of searches, and the run time is
		// thus guaranteed to be linear.
		baseOff += j + n
		result = result[i+n:]
		base = base[j+n:]
	}
	for len(result) > 0 {
		n := len(result)
		if n > maxInsertLen {
			n = maxInsertLen
		}
		delta = append(append(delta, byte(n)), result[:n]...)
		result = result[n:]
	}
	return
}

// longestCommonSubstring returns the respective starting positions and
// the length of the longest common substring between the slices a and
// b.  It returns -1, -1, 0 if no common substring exists.  The function
// operates in O(len(a)*len(b)) time and O(min(len(a), len(b))) space.
func longestCommonSubstring(a, b []byte) (ai, bj, n int) {
	if len(b) < len(a) {
		bj, ai, n = longestCommonSubstring(b, a)
		return
	}
	c := make([]int, len(a))
	for j := range b {
		d := 0
		for i := range a {
			tmp := c[i]
			if a[i] == b[j] {
				c[i] = d + 1
				if c[i] > n {
					ai = i
					bj = j
					n = c[i]
				}
			} else {
				c[i] = 0
			}
			d = tmp
		}
	}
	ai -= n - 1
	bj -= n - 1
	return
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

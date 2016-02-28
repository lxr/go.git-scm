// Package delta implements version 3 Git packfile delta objects.
// A delta object encodes a sequence of instructions for generating a
// new Git object based on an old one.  For details on their binary
// representation, see http://git.rsbx.net/Documents/Git_Data_Formats.txt.
//
// In a packfile, the (deflated) body of a delta object is preceded
// by a reference to its base object as well as a number encoding the
// type of this reference.  This package concerns itself with pure delta
// data and leaves parsing and resolving such references to the client
// code.  For typographical convenience, however, constants for the two
// types of reference used in Git packfiles are defined in this package.
package delta

import (
	"bytes"
	"errors"
	"io"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/packfile/base128"
	"github.com/lxr/go.git-scm/packfile/internal"
)

// The "type" of a delta object defines how its base object is
// referenced: with its byte offset from the current position within
// the packfile stream, or by its object ID.
const (
	TypeOffset object.Type = 6
	TypeRef    object.Type = 7
)

// ErrApply is returned by Object.Apply if pre- or post-apply sanity
// checks fail.
var ErrApply = errors.New("delta does not apply cleanly")

// An Object is a delta object.
type Object struct {
	baseLen   int
	resultLen int
	ops       opList
}

// Apply applies the delta to a base object and returns the result as a
// new object.
func (d Object) Apply(base object.Interface) (object.Interface, error) {
	data, err := internal.MarshalObj(base)
	if err != nil {
		return nil, err
	}
	src := bytes.NewReader(data)
	if len(data) != d.baseLen {
		return nil, ErrApply
	}
	dst := new(bytes.Buffer)
	if err = d.ops.Apply(dst, src); err != nil {
		return nil, err
	}
	data = dst.Bytes()
	if len(data) != d.resultLen {
		return nil, ErrApply
	}
	result, _ := object.New(object.TypeOf(base))
	return result, internal.UnmarshalObj(result, data)
}

// Unmarshal decodes a delta object from the Git packfile format.
func Unmarshal(data []byte) (d Object, err error) {
	buf := bytes.NewBuffer(data)
	baseLen, err := base128.ReadLE(buf)
	if err != nil {
		return
	}
	resultLen, err := base128.ReadLE(buf)
	if err != nil {
		return
	}
	d.baseLen, d.resultLen = int(baseLen), int(resultLen)
	for buf.Len() > 0 {
		var opcode, c byte
		opcode, err = buf.ReadByte()
		if err != nil {
			return
		}
		var op op
		switch opcode >> 7 {
		case 0: // insert
			ins := make(insertOp, opcode)
			if _, err = io.ReadFull(buf, ins); err != nil {
				return
			}
			op = ins
		case 1: // copy
			var cp copyOp
			for i := uint(0); i < 4; i++ {
				if opcode&1 == 1 {
					c, err = buf.ReadByte()
					if err != nil {
						return
					}
					cp.Off |= int64(c) << (i * 8)
				}
				opcode >>= 1
			}
			for i := uint(0); i < 3; i++ {
				if opcode&1 == 1 {
					c, err = buf.ReadByte()
					if err != nil {
						return
					}
					cp.Len |= int64(c) << (i * 8)
				}
				opcode >>= 1
			}
			if cp.Len == 0 {
				cp.Len = 1 << 16
			}
			op = cp
		default:
			panic("byte has more than 8 bits")
		}
		d.ops = append(d.ops, op)
	}
	return
}

// An op represents one delta operation.  The Apply method writes bytes
// to the destination buffer, reading from the source buffer if
// necessary, and returns any possible error.
type op interface {
	Apply(dst io.Writer, src io.ReaderAt) error
}

// An insertOp simply writes its bytes to the destination buffer.
type insertOp []byte

func (i insertOp) Apply(dst io.Writer, src io.ReaderAt) error {
	_, err := dst.Write(i)
	return err
}

// A copyOp copies Len bytes to the destination buffer from the source
// at offset Off.
type copyOp struct {
	Off int64
	Len int64
}

func (c copyOp) Apply(dst io.Writer, src io.ReaderAt) error {
	_, err := io.Copy(dst, io.NewSectionReader(src, c.Off, c.Len))
	return err
}

// An opList is a sequence of delta operations.
type opList []op

func (ol opList) Apply(dst io.Writer, src io.ReaderAt) error {
	for _, op := range ol {
		if err := op.Apply(dst, src); err != nil {
			return err
		}
	}
	return nil
}

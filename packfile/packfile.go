// Package packfile provides support for reading and writing version 3
// Git packfiles.  See http://git.rsbx.net/Documents/Git_Data_Formats.txt
// for details.  Version 2 packfiles can also be read, but they will
// fail with an unhelpful error message if they use the version 2
// -specific delta object copy mode that copies from the result buffer
// instead of the source one.  (No known packfiles use this option,
// however.)
package packfile

// BUG(lor): "Thin" packfiles are not supported: a packfile must contain
// the base objects of all its deltas so that they can be resolved
// without recourse to a repository.

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"hash"
	"io"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/packfile/base128"
	"github.com/lxr/go.git-scm/packfile/delta"
	"github.com/lxr/go.git-scm/packfile/internal"
)

var (
	// ErrBadBase is returned when reading packfile data where the
	// base offset or ID of a delta object does not refer to an
	// earlier object in the stream.
	ErrBadBase = errors.New("packfile: unknown base for delta object")
	// ErrChecksum is returned when reading packfile data that has
	// an invalid checksum.
	ErrChecksum = errors.New("packfile: invalid checksum")
	// ErrHeader is returned when reading packfile data that has
	// an invalid header.
	ErrHeader = errors.New("packfile: invalid header")
	// ErrTooManyObjects is returned when creating a packfile with
	// an invalid number of objects, or when writing too many
	// objects into one.
	ErrTooManyObjects = errors.New("packfile: too many objects")
	// ErrVersion is returned when reading packfile data with a
	// version number other than 2 or 3.
	ErrVersion = errors.New("packfile: unsupported version")
)

var signature = [4]byte{'P', 'A', 'C', 'K'}

type header struct {
	Signature [4]byte
	Version   uint32
	Nobjects  uint32
}

// A Reader reads Git objects from a packfile stream.
type Reader struct {
	r      *posReader
	n      int64
	digest hash.Hash

	// XXX(lor): A Reader must maintain a private reference to all
	// objects it has read, as any one of them can potentially be
	// the base object of a future delta.  The cost in memory is
	// unfortunate, but I can think of no alternative that wouldn't
	// either complicate the implementation or require packfiles to
	// always go hand-in-hand with repositories.
	//
	// Memoing objects as object.Interfaces is also a bit silly, as
	// resolving a delta requires marshaling the base object into a
	// byte slice to start with.  This results in a lot of pointless
	// marshaling/unmarshaling overhead, but the alternative is to
	// expose a byte-slice-to-byte-slice interface in
	// packfile/delta, which I don't really want to do, as Git
	// deltas are semantically computed between Git objects and not
	// random blobs of bytes.
	ofs map[int64]object.Interface
	ref map[object.ID]object.Interface
}

// NewReader creates a new Reader from r.  It returns an error if r
// does not begin with a packfile header, if the packfile version is
// unsupported, or if trying to read the header failed.
func NewReader(r io.Reader) (*Reader, error) {
	p := new(Reader)
	p.digest = sha1.New()
	p.r = &posReader{r: io.TeeReader(r, p.digest)}
	var h header
	err := binary.Read(p.r, binary.BigEndian, &h)
	switch {
	case err != nil:
		err = err
	case h.Signature != signature:
		err = ErrHeader
	case h.Version < 2 || h.Version > 3:
		err = ErrVersion
	}
	p.n = int64(h.Nobjects)
	p.ofs = make(map[int64]object.Interface)
	p.ref = make(map[object.ID]object.Interface)
	return p, err
}

// Len returns the number of objects remaining in the packfile.
func (r *Reader) Len() int64 {
	return r.n
}

// Read returns the next object in the stream, or nil, io.EOF if there
// are no more objects.  It returns nil, ErrChecksum if the packfile
// ends with an invalid checksum.
func (r *Reader) Read() (obj object.Interface, err error) {
	if r.n > 0 {
		obj, err = r.readObject()
		if err == nil {
			r.n--
		}
	} else {
		err = r.readChecksum()
		if err == nil {
			err = io.EOF
		}
	}
	return
}

// readObject returns the next object in the stream.
func (r *Reader) readObject() (obj object.Interface, err error) {
	pos := r.r.Tell()
	objType, size, err := readObjHeader(r.r)
	if err != nil {
		return
	}

	ok := true
	var base object.Interface
	switch objType {
	case delta.TypeOffset:
		var negOfs uint64
		negOfs, err = base128.ReadMBE(r.r)
		if int64(negOfs) < 0 {
			err = errors.New("packfile: delta offset overflows int64")
			break
		}
		base, ok = r.ofs[pos-int64(negOfs)]
	case delta.TypeRef:
		var baseID object.ID
		_, err = io.ReadFull(r.r, baseID[:])
		base, ok = r.ref[baseID]
	}
	switch {
	case err != nil:
		return
	case !ok:
		err = ErrBadBase
		return
	}

	zr, err := zlib.NewReader(r.r)
	if err != nil {
		return
	}
	defer zr.Close()
	data := make([]byte, size)
	if _, err = io.ReadFull(zr, data); err != nil {
		return
	}
	// If one reads the exact length of the compressed data from a
	// zlib.Reader, as above, the zlib checksum isn't read, and the
	// packfile stream is thus thrown out of sync.  One needs to
	// read "past" the end of the data to get zlib to read and check
	// the checksum.
	var dummy [4]byte
	zr.Read(dummy[:])

	if base != nil {
		var d delta.Object
		d, err = delta.Unmarshal(data)
		if err != nil {
			return
		}
		obj, err = d.Apply(base)
		if err != nil {
			return
		}
	} else {
		obj, err = object.New(objType)
		if err != nil {
			return
		}
		err = internal.UnmarshalObj(obj, data)
		if err != nil {
			return
		}
	}

	id, err := object.Hash(obj)
	if err != nil {
		return
	}
	r.ofs[pos] = obj
	r.ref[id] = obj
	return
}

// readChecksum reads the SHA-1 footer of a packfile and compares it to
// the checksum accumulated by NewReader and the readObject calls.
func (r *Reader) readChecksum() error {
	var my, other [sha1.Size]byte
	copy(my[:], r.digest.Sum(nil))
	_, err := io.ReadFull(r.r, other[:])
	if err == nil && my != other {
		err = ErrChecksum
	}
	return err
}

// A Writer writes Git objects to a packfile stream.
type Writer struct {
	w      *posWriter
	n      int64
	digest hash.Hash
}

// NewWriter creates a new Writer from w.  n is the number of objects
// that the packfile will contain.  NewWriter returns a non-nil error
// if it fails to write the packfile header or if n is outside the range
// of an unsigned 32-bit integer.
func NewWriter(w io.Writer, n int64) (*Writer, error) {
	if int64(uint32(n)) != n {
		return nil, ErrTooManyObjects
	}
	pfw := new(Writer)
	pfw.n = n
	pfw.digest = sha1.New()
	pfw.w = &posWriter{w: io.MultiWriter(w, pfw.digest)}
	h := header{signature, 3, uint32(n)}
	return pfw, binary.Write(pfw.w, binary.BigEndian, h)
}

// BUG(lor): Writer.Write writes all its arguments as full objects;
// it does not attempt to delta compress them.

// Write writes a Git object to the stream.  It returns
// nil, ErrTooManyObjects if trying to write more objects than were
// specified in the call to NewWriter.
func (w *Writer) Write(obj object.Interface) error {
	if w.n == 0 {
		return ErrTooManyObjects
	}
	data, err := internal.MarshalObj(obj)
	if err != nil {
		return err
	}
	err = writeObjHeader(w.w, object.TypeOf(obj), int64(len(data)))
	if err != nil {
		return err
	}
	z := zlib.NewWriter(w.w)
	if _, err = z.Write(data); err != nil {
		return err
	}
	w.n--
	return z.Close()
}

// Close writes the packfile SHA-1 footer to the stream.  It does not
// close the underlying writer.
func (w *Writer) Close() error {
	_, err := w.w.Write(w.digest.Sum(nil))
	return err
}

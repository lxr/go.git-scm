// Package packfile provides support for reading and writing version 3
// Git packfiles.  See http://git.rsbx.net/Documents/Git_Data_Formats.txt
// for details.  Version 2 packfiles can also be read, but they will
// fail with an unhelpful error message if they use the version 2
// -specific delta object copy mode that copies from the result buffer
// instead of the source one.  (It is unknown if any packfiles actually
// use this option, however.)
package packfile

import (
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"io"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/repository"
)

// If one happens to read the exact length of the compressed data from a
// zlib reader, the zlib checksum isn't read.  flushZlib can be used to
// read "past" the end of the zlib stream in order to consume and check
// the checksum.
func flushZlib(r io.Reader) error {
	if _, err := io.ReadFull(r, []byte{0}); err != io.EOF {
		return err
	} else if err == nil {
		return errors.New("zlib stream not at end")
	}
	return nil
}

var (
	// ErrBadOffset is returned when reading packfile data where
	// the base offset of an ofs-delta object does not refer to an
	// earlier object in the stream.
	ErrBadOffset = errors.New("packfile: delta base offset does not point at an object boundary")
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
	r    *digestReader
	n    int64
	ofs  map[int64]object.ID
	repo repository.Interface
}

// NewReader creates a new Reader from r.  It returns an error if r
// does not begin with a packfile header, if the packfile version is
// unsupported, or if trying to read the header failed.  repo will be
// used to resolve delta objects into full ones; it must contain all
// potential base objects.  (The caller may have to enforce this by
// adding each read object to the repository.)  It is the caller's
// responsibility to call Close on the Reader after all objects have
// been read.
func NewReader(r io.Reader, repo repository.Interface) (*Reader, error) {
	dr := newDigestReader(r, sha1.New())
	var h header
	err := binary.Read(dr, binary.BigEndian, &h)
	switch {
	case err != nil:
		return nil, err
	case h.Signature != signature:
		return nil, ErrHeader
	case h.Version < 2 || h.Version > 3:
		return nil, ErrVersion
	}
	return &Reader{
		r:    dr,
		n:    int64(h.Nobjects),
		ofs:  make(map[int64]object.ID),
		repo: repo,
	}, nil
}

// Len returns the number of objects remaining in the packfile.
func (r *Reader) Len() int64 {
	return r.n
}

// Read returns the next object in the stream, or nil, io.EOF if there
// are no more objects.
func (r *Reader) Read() (obj object.Interface, err error) {
	// check if there are objects to read, and if so, record the
	// current position as the start of a new object
	if r.n == 0 {
		return nil, io.EOF
	}
	pos := r.r.Tell()

	// read object header
	objType, size, err := readObjHeader(r.r)
	if err != nil {
		return
	}

	// if object is a delta, read its base object reference
	var baseID object.ID
	switch objType {
	case offsetDelta:
		negOfs, err := readBase128MBE(r.r)
		switch {
		case err != nil:
			return nil, err
		case int64(negOfs) < 0:
			return nil, errors.New("packfile: delta offset overflows int64")
		}
		var ok bool
		baseID, ok = r.ofs[pos-int64(negOfs)]
		if !ok {
			return nil, ErrBadOffset
		}
	case refDelta:
		if _, err = io.ReadFull(r.r, baseID[:]); err != nil {
			return
		}
	}

	// read object body
	zr, err := zlib.NewReader(r.r)
	if err != nil {
		return
	}
	defer zr.Close()
	data := make([]byte, size)
	if _, err = io.ReadFull(zr, data); err != nil {
		return
	}
	if err = flushZlib(zr); err != nil {
		return
	}

	// if object is a delta, retrieve its base object and apply
	// the delta to it
	if baseID != object.ZeroID {
		var (
			base     object.Interface
			baseData []byte
		)
		base, err = r.repo.GetObject(baseID)
		if err != nil {
			return
		}
		baseData, err = marshalObj(base)
		if err != nil {
			return
		}
		data, err = applyDelta(baseData, data)
		if err != nil {
			return
		}
		objType = object.TypeOf(base)
	}

	// unmarshal the object
	obj, err = object.New(objType)
	if err != nil {
		return
	}
	err = unmarshalObj(obj, data)
	if err != nil {
		return
	}

	// update bookkeeping data and return the object
	r.ofs[pos] = hashObj(objType, data)
	r.n--
	return
}

// Close reads and verifies the packfile SHA-1 footer from the stream.
// It returns ErrChecksum if the checksum is not valid.  It does not
// close the underlying reader.  This method should only be called after
// all objects have been read.
func (r *Reader) Close() error {
	var read, expected [sha1.Size]byte
	copy(expected[:], r.r.Sum(nil))
	_, err := io.ReadFull(r.r, read[:])
	switch {
	case err != nil:
		return err
	case read != expected:
		return ErrChecksum
	}
	return nil
}

// A Writer writes Git objects to a packfile stream.
type Writer struct {
	w *digestWriter
	n int64
}

// NewWriter creates a new Writer from w.  n is the number of objects
// that the packfile will contain.  NewWriter returns a non-nil error
// if it fails to write the packfile header or if n is outside the range
// of an unsigned 32-bit integer.  It is the caller's responsibility to
// call Close on the Writer after all objects have been written.
func NewWriter(w io.Writer, n int64) (*Writer, error) {
	if int64(uint32(n)) != n {
		return nil, ErrTooManyObjects
	}
	dw := newDigestWriter(w, sha1.New())
	h := header{signature, 3, uint32(n)}
	if err := binary.Write(dw, binary.BigEndian, h); err != nil {
		return nil, err
	}
	return &Writer{dw, n}, nil
}

// Len returns the number of objects that still need to be written to
// the packfile.
func (w *Writer) Len() int64 {
	return w.n
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
	data, err := marshalObj(obj)
	if err != nil {
		return err
	}
	err = writeObjHeader(w.w, object.TypeOf(obj), int64(len(data)))
	if err != nil {
		return err
	}
	z := zlib.NewWriter(w.w)
	if _, err = z.Write(data); err != nil {
		z.Close()
		return err
	}
	w.n--
	return z.Close()
}

// Close writes the packfile SHA-1 footer to the stream.  It does not
// close the underlying writer.  This method should only be called after
// all objects have been written.
func (w *Writer) Close() error {
	_, err := w.w.Write(w.w.Sum(nil))
	return err
}

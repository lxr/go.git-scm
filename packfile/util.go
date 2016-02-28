package packfile

import (
	"errors"
	"io"

	"github.com/lxr/go.git-scm/object"
	"github.com/lxr/go.git-scm/packfile/base128"
)

// A packfile object header is a little-endian base128-encoded number
// where bits 4-6 encode the object's type and the rest its size.

func readObjHeader(r io.ByteReader) (object.Type, int64, error) {
	hdr, err := base128.ReadLE(r)
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
	return base128.WriteLE(w, hdr)
}

// posReader is a reader which records the current position in the
// stream.  As a local convenience, it also provides a ReadByte()
// wrapper around the reader.
type posReader struct {
	r   io.Reader
	pos int64
}

func newPosReader(r io.Reader) *posReader {
	return &posReader{r, 0}
}

func (r *posReader) Tell() int64 {
	return r.pos
}

func (r *posReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.pos += int64(n)
	return n, err
}

func (r *posReader) ReadByte() (byte, error) {
	b := make([]byte, 1)
	_, err := io.ReadFull(r, b)
	return b[0], err
}

// posWriter is a writer that records how many bytes have been written
// to the stream.
type posWriter struct {
	w   io.Writer
	pos int64
}

func newPosWriter(w io.Writer) *posWriter {
	return &posWriter{w, 0}
}

func (w *posWriter) Tell() int64 {
	return w.pos
}

func (w *posWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.pos += int64(n)
	return n, err
}

package packfile

import (
	"bufio"
	"compress/flate"
	"errors"
	"hash"
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
	_, err := base128.WriteLE(w, hdr)
	return err
}

// A digestReader tracks the number and checksum of bytes read from an
// underlying io.Reader.  As a local convenience, it also implements
// io.ByteReader.  If the underlying io.Reader does not implement the
// interface natively, it is wrapped in a bufio.Reader.  Note that this
// may cause more bytes to be read from the io.Reader than digestReader
// will report.
type digestReader struct {
	r      flate.Reader
	pos    int64
	digest hash.Hash
}

func newDigestReader(r io.Reader, h hash.Hash) *digestReader {
	fr, ok := r.(flate.Reader)
	if !ok {
		fr = bufio.NewReader(r)
	}
	return &digestReader{fr, 0, h}
}

func (r *digestReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.pos += int64(n)
	if err == nil {
		_, err = r.digest.Write(p[:n])
	}
	return n, err
}

func (r *digestReader) ReadByte() (byte, error) {
	c, err := r.r.ReadByte()
	if err == nil {
		r.pos++
		_, err = r.digest.Write([]byte{c})
	}
	return c, err
}

func (r *digestReader) Sum(b []byte) []byte {
	return r.digest.Sum(b)
}

func (r *digestReader) Tell() int64 {
	return r.pos
}

// A digestWriter tracks the number and checksum of bytes written to an
// underlying io.Writer.
type digestWriter struct {
	w      io.Writer
	pos    int64
	digest hash.Hash
}

func newDigestWriter(w io.Writer, h hash.Hash) *digestWriter {
	return &digestWriter{w, 0, h}
}

func (w *digestWriter) Sum(b []byte) []byte {
	return w.digest.Sum(b)
}

func (w *digestWriter) Tell() int64 {
	return w.pos
}

func (w *digestWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.pos += int64(n)
	if err == nil {
		_, err = w.digest.Write(p[:n])
	}
	return n, err
}

// Git packfiles end with an SHA-1 checksum of their contents.  The
// starting offsets of objects within the packfile also need to be
// recorded for delta resolution.  This file defines convenience types
// for reading from and writing to files while maintaining this
// information.

package packfile

import (
	"bufio"
	"compress/flate"
	"hash"
	"io"
)

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
	r.digest.Write(p[:n])
	return n, err
}

func (r *digestReader) ReadByte() (byte, error) {
	c, err := r.r.ReadByte()
	if err != nil {
		return 0, err
	}
	r.pos++
	r.digest.Write([]byte{c})
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
	w.digest.Write(p[:n])
	return n, err
}

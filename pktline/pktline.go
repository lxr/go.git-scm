// Package pktline provides support for reading and writing the Git
// pkt-line wire protocol.  See https://www.kernel.org/pub/software/scm/git/docs/technical/protocol-common.html#_pkt_line_format
// for details.
package pktline

import (
	"errors"
	"fmt"
	"io"
)

func min(a, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func readBytes(r io.Reader, n int) ([]byte, error) {
	p := make([]byte, n)
	n, err := io.ReadFull(r, p)
	return p[:n], err
}

// MaxPayloadLen is the maximum length of a pkt-line payload.
const MaxPayloadLen = 65520

// ErrTooLong is returned by Writer.Write if the payload length exceeds
// MaxPayloadLen.
var ErrTooLong = errors.New("pkt-line too long")

// A Reader reads pkt-line records from an underlying reader.
// The method Next must be called to start reading the first pkt-line
// substream.
type Reader struct {
	r    io.Reader
	want int
}

// NewReader creates a new Reader from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r, -1}
}

// readLineLen reads the length of the next pkt-line in the stream and
// stores it in r.want, substracting from it the four bytes of the
// length itself.
func (r *Reader) readLineLen() error {
	_, err := fmt.Fscanf(r.r, "%04x", &r.want)
	r.want -= 4
	return err
}

// Len returns the number of bytes remaining on the current pkt-line.
// Zero is returned only at the start of an empty pkt-line.  Len returns
// a negative number at a flush-pkt.
func (r *Reader) Len() int {
	return r.want
}

// Next advances the reader to the next pkt-line substream (including
// the first), skipping any remaining pkt-lines in the current
// substream.  It returns io.EOF if at the end of the underlying reader.
func (r *Reader) Next() error {
	for r.want >= 0 {
		if _, err := r.ReadMsg(); err != nil {
			return err
		}
	}
	err := r.readLineLen()
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return err
}

// Read reads bytes from the pkt-line stream.  Reads are truncated at
// pkt-line boundaries, so it is very likely for Read to return
// n < len(p) with a nil error.  (A consequence of this is that an empty
// pkt-line in a stream results in one read returning 0, nil.)  If a
// read ends at a pkt-line boundary, whether naturally or through
// truncation, Read prereads the length of the next pkt-line, and any
// error in doing this is returned in err.  Read returns 0, io.EOF after
// it encounters a flush-pkt until Next is called.
func (r *Reader) Read(p []byte) (n int, err error) {
	if r.want < 0 {
		return 0, io.EOF
	}
	n = min(len(p), r.want)
	n, err = io.ReadFull(r.r, p[:n])
	r.want -= n
	if r.want == 0 && err == nil {
		err = r.readLineLen()
	}
	return n, err
}

// ReadMsg returns the remaining pkt-line as a byte slice.  An empty
// slice is returned only at the start of an empty pkt-line.  ReadMsg
// returns nil, io.EOF after it encounters a flush-pkt until Next is
// called.  On error, ReadMsg returns whatever was read of the pkt-line.
//
// The name of this method is likely to change if an interface with
// similar semantics is added to the Go standard library.
func (r *Reader) ReadMsg() ([]byte, error) {
	if n := r.Len(); n < 0 {
		return nil, io.EOF
	} else {
		return readBytes(r, n)
	}
}

// ReadMsgString behaves like ReadMsg, except it returns the pkt-line
// as a string.
func (r *Reader) ReadMsgString() (string, error) {
	p, err := r.ReadMsg()
	return string(p), err
}

// A Writer writes pkt-line records to an underlying writer.
type Writer struct {
	w io.Writer
}

// NewWriter creates a new Writer from w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w}
}

// Flush sends a flush-pkt to the underlying writer.
func (w *Writer) Flush() error {
	_, err := w.w.Write([]byte("0000"))
	return err
}

// Writer writes p as a single pkt-line record.  It returns
// 0, ErrTooLong if len(p) exceeds MaxPayloadLen.
func (w *Writer) Write(p []byte) (int, error) {
	if len(p) > MaxPayloadLen {
		return 0, ErrTooLong
	}
	if _, err := fmt.Fprintf(w.w, "%04x", len(p)+4); err != nil {
		return 0, err
	}
	return w.w.Write(p)
}

// WriteString writes s as a single pkt-line record.  It behaves like
// Write.
func (w *Writer) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

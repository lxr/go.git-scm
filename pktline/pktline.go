// Package pktline provides support for reading and writing the Git
// pkt-line wire protocol.  See https://www.kernel.org/pub/software/scm/git/docs/technical/protocol-common.html#_pkt_line_format
// for details.
package pktline

import (
	"errors"
	"io"
)

// MaxPayloadLen is the maximum length of a pkt-line payload.
const MaxPayloadLen = 65520

// ErrTooLong is returned by Writer.WriteLine if the payload length
// exceeds MaxPayloadLen.
var ErrTooLong = errors.New("pkt-line too long")

// readLineLen reads a pkt-line length from r.  The four bytes of the
// length itself are subtracted from the returned value.
func readLineLen(r io.Reader) (int, error) {
	var p [4]byte
	if _, err := io.ReadFull(r, p[:]); err != nil {
		return 0, err
	}
	var x int
	for i, c := range p {
		switch c {
		case 'A', 'B', 'C', 'D', 'E', 'F':
			x |= int(c-'A'+10) << (uint(4-i-1) * 4)
		case 'a', 'b', 'c', 'd', 'e', 'f':
			x |= int(c-'a'+10) << (uint(4-i-1) * 4)
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			x |= int(c-'0') << (uint(4-i-1) * 4)
		}
	}
	return x - 4, nil
}

// writeLineLen writes a pkt-line length x to w.  The four bytes of the
// length itself are added to x.
func writeLineLen(w io.Writer, x int) error {
	hexDigits := []byte("0123456789abcdef")
	x += 4
	p := [4]byte{
		hexDigits[x>>12&0xF],
		hexDigits[x>>8&0xF],
		hexDigits[x>>4&0xF],
		hexDigits[x>>0&0xF],
	}
	_, err := w.Write(p[:])
	return err
}

// A Reader reads pkt-line records from an underlying reader.
type Reader struct {
	r     io.Reader
	atEOF bool
}

// NewReader creates a new Reader from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{r, false}
}

// Next advances the Reader past a flush-pkt.  It should only be called
// after Read has returned io.EOF.
func (r *Reader) Next() {
	r.atEOF = false
}

// ReadLine reads and returns the next pkt-line in the stream.
// On error, it returns what was successfully read of the pkt-line.
// This error is io.ErrUnexpectedEOF if an EOF is encountered in the
// middle of a pkt-line.  ReadLine returns "", io.EOF at a flush-pkt
// until Next is called.
func (r *Reader) ReadLine() (string, error) {
	n, err := readLineLen(r.r)
	switch {
	case err != nil:
		return "", err
	case n < 0: // flush-pkt
		r.atEOF = true
		return "", io.EOF
	}
	p := make([]byte, n)
	n, err = io.ReadFull(r.r, p)
	return string(p[:n]), err
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

// WriteLine writes s as a single pkt-line record.  It returns
// ErrTooLong if len(s) exceeds MaxPayloadLen.
func (w *Writer) WriteLine(s string) error {
	if len(s) > MaxPayloadLen {
		return ErrTooLong
	}
	if err := writeLineLen(w.w, len(s)); err != nil {
		return err
	}
	_, err := io.WriteString(w.w, s)
	return err
}

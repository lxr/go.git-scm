// Functionality shared between Commit and Tag objects.

package object

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// A safeString is a string that may not contain the bytes [\x00\x0A<>]
// nor begin nor end with the bytes [ .,:;<>"'].  safeStrings are used
// as author names and e-mail addresses in Git.
type safeString string

func (s *safeString) Scan(ss fmt.ScanState, verb rune) error {
	safe, err := getToken(ss, "\x00\x0A<>", " .,:;<>\"'")
	if err != nil {
		return err
	}
	*s = safeString(safe)
	return nil
}

// getToken finds the longest prefix of ss that contains no runes in
// delimset and returns it trimmed of all leading and trailing runes in
// trimset.
func getToken(ss fmt.ScanState, delimset, trimset string) (string, error) {
	p := func(r rune) bool { return !strings.ContainsRune(delimset, r) }
	tok, err := ss.Token(false, p)
	if err != nil {
		return "", err
	}
	return strings.Trim(string(tok), trimset), nil
}

// A Signature tells the author and date of a Git commit or tag.
type Signature struct {
	Name  string
	Email string
	Date  time.Time
}

// String returns the Signature in the format "Name <Email> Date",
// where Date is formatted as the Unix time followed by a space and
// a four-digits-plus-sign timezone offset.
func (s Signature) String() string {
	return fmt.Sprintf("%s <%s> %d %s",
		s.Name,
		s.Email,
		s.Date.Unix(),
		s.Date.Format("-0700"),
	)
}

// Scan is a support routine for fmt.Scanner.  The format verb is
// ignored; Scan always attempts to read a signature string as returned
// by String from the input.
func (s *Signature) Scan(ss fmt.ScanState, verb rune) error {
	var (
		name   safeString
		email  safeString
		unix   int64
		offset int
	)
	// XXX(lor): There should be a space between %s and <%s> in the
	// format string, but unfortunately scanning a safeString stops
	// only at an angle bracket, at which point it's too late to
	// unread the preceding space back into the scanner.
	_, err := fmt.Fscanf(ss, "%s<%s> %d %05d",
		&name,
		&email,
		&unix,
		&offset,
	)
	if err != nil {
		return err
	}
	offset = (offset/100)*60*60 + (offset%100)*60
	s.Name = string(name)
	s.Email = string(email)
	s.Date = time.Unix(unix, 0).In(time.FixedZone("", offset))
	return nil
}

// A fmtErr can be used to cache errors from fmt function calls for
// later inspection, while passing the byte/argument counts they return
// through unmolested.
type fmtErr struct {
	err error
}

func (e *fmtErr) Check(n int, err error) int {
	if e.err == nil {
		e.err = err
	}
	return n
}

func (e *fmtErr) Err() error {
	return e.err
}

// prependHeader prepends a Git object header to an object's binary
// representation.  It returns a TypeError containing the objType
// argument if it is not one of the standard Git ones.
func prependHeader(objType Type, data []byte) ([]byte, error) {
	if objType.String() == "" {
		return nil, &TypeError{objType}
	}
	header := []byte(fmt.Sprintf("%s %d\x00", objType, len(data)))
	return append(header, data...), nil
}

// stripHeader strips the Git object header from an object's binary
// representation and validates the type and length recorded in it.
// It returns the remaining data in the representation.  It returns a
// TypeError containing the objType argument if it is not one of the
// standard Git ones.
func stripHeader(objType Type, data []byte) ([]byte, error) {
	if objType.String() == "" {
		return nil, &TypeError{objType}
	}
	buf := bytes.NewBuffer(data)
	var bufType Type
	var length int
	_, err := fmt.Fscanf(buf, "%s %d\x00", &bufType, &length)
	switch {
	case err != nil:
		return nil, err
	case bufType != objType:
		return nil, fmt.Errorf("object: expected type %s, got %s", objType, bufType)
	case length != buf.Len():
		return nil, fmt.Errorf("object: expected length %d, got %d", length, buf.Len())
	default:
		return buf.Bytes(), err
	}
}

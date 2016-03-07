// Functionality shared between Commit and Tag objects.

package object

import (
	"bytes"
	"fmt"
	"reflect"
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

// defaultMarshalText is the common function for serializing Commit and
// Tag objects.  It uses reflection to read all fields of *v except the
// last and print them with one field per line, with the lower-cased
// field name and a space prefixed to the field value.  The contents of
// the last field of *v are printed after a blank line.
func defaultMarshalText(v interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	o := reflect.ValueOf(v).Elem()
	t := o.Type()
	n := t.NumField() - 1
	for i := 0; i < n; i++ {
		name := strings.ToLower(t.Field(i).Name)
		v := o.Field(i)
		switch v.Kind() {
		case reflect.Slice:
			for j := 0; j < v.Len(); j++ {
				fmt.Fprintln(buf, name, v.Index(j).Interface())
			}
		default:
			fmt.Fprintln(buf, name, v.Interface())
		}
	}
	buf.WriteString("\n" + o.Field(n).String())
	return buf.Bytes(), nil
}

// defaultUnmarshalText is the common function for deserializing Commit
// and Tag objects.  It reads lines of text from data and tries to
// store them in the field of *v indicated by the first space-separated
// word on the line.  Everything after the first blank line is stored
// in *v's last field.
func defaultUnmarshalText(data []byte, v interface{}) error {
	o := reflect.ValueOf(v).Elem()
	buf := bytes.NewBuffer(data)
	for {
		var name string
		if _, err := fmt.Fscanf(buf, "%s ", &name); err != nil {
			break
		}
		f := o.FieldByName(strings.Title(name))
		if !f.IsValid() {
			return fmt.Errorf("unknown field in Git object text: %s", name)
		}
		var err error
		switch f.Kind() {
		case reflect.Slice:
			x := reflect.New(f.Type().Elem())
			_, err = fmt.Fscanln(buf, x.Interface())
			f.Set(reflect.Append(f, x.Elem()))
		default:
			_, err = fmt.Fscanln(buf, f.Addr().Interface())
		}
		if err != nil {
			return err
		}
	}
	f := o.Field(o.NumField() - 1)
	f.SetString(buf.String())
	return nil
}

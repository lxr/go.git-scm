package object

import (
	"bytes"
	"fmt"
)

// A Tag is a named label for another Git object, usually a Commit.
type Tag struct {
	Object  ID        // ID of the tagged object
	Type    Type      // type of the tagged object
	Tag     string    // tag name
	Tagger  Signature // tagger name and date
	Message string    // a tag message
}

func (t *Tag) MarshalBinary() ([]byte, error) {
	text, err := t.MarshalText()
	if err != nil {
		return nil, err
	}
	return prependHeader(TypeTag, text)
}

func (t *Tag) UnmarshalBinary(data []byte) error {
	text, err := stripHeader(TypeTag, data)
	if err != nil {
		return err
	}
	return t.UnmarshalText(text)
}

func (t *Tag) MarshalText() ([]byte, error) {
	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "object", t.Object)
	fmt.Fprintln(buf, "type", t.Type)
	fmt.Fprintln(buf, "tag", t.Tag)
	fmt.Fprintln(buf, "tagger", t.Tagger)
	fmt.Fprintln(buf)
	buf.WriteString(t.Message)
	return buf.Bytes(), nil
}

func (t *Tag) UnmarshalText(text []byte) error {
	buf := bytes.NewBuffer(text)
	var err fmtErr
	err.Check(fmt.Fscanf(buf, "object %s\n", &t.Object))
	err.Check(fmt.Fscanf(buf, "type %s\n", &t.Type))
	err.Check(fmt.Fscanf(buf, "tag %s\n", &t.Tag))
	err.Check(fmt.Fscanf(buf, "tagger %s\n", &t.Tagger))
	err.Check(fmt.Fscanf(buf, "\n"))
	t.Message = buf.String()
	return err.Err()
}

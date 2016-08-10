package object

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
	return defaultMarshalText(t)
}

func (t *Tag) UnmarshalText(text []byte) error {
	return defaultUnmarshalText(text, t)
}

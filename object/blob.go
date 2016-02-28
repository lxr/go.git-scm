package object

// A Blob represents the contents of a file.  It is its own textual
// representation.
type Blob []byte

func (b *Blob) MarshalBinary() ([]byte, error) {
	text, _ := b.MarshalText()
	return prependHeader(TypeBlob, text)
}

func (b *Blob) UnmarshalBinary(data []byte) error {
	text, err := stripHeader(TypeBlob, data)
	if err != nil {
		return err
	}
	return b.UnmarshalText(text)
}

func (b *Blob) MarshalText() ([]byte, error) {
	text := make([]byte, len(*b))
	copy(text, *b)
	return text, nil
}

func (b *Blob) UnmarshalText(text []byte) error {
	*b = make(Blob, len(text))
	copy(*b, text)
	return nil
}

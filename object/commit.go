package object

// A Commit is a signed label for a Tree object, representing a snapshot
// of the repository state at a particular point in time.
type Commit struct {
	Tree      ID        // ID of the commit's root tree
	Parent    []ID      // the commit's parents
	Author    Signature // author name and date
	Committer Signature // committer name and date
	Message   string    // a commit message
}

func (c *Commit) MarshalBinary() ([]byte, error) {
	text, err := c.MarshalText()
	if err != nil {
		return nil, err
	}
	return prependHeader(TypeCommit, text)
}

func (c *Commit) UnmarshalBinary(data []byte) error {
	text, err := stripHeader(TypeCommit, data)
	if err != nil {
		return err
	}
	return c.UnmarshalText(text)
}

func (c *Commit) MarshalText() ([]byte, error) {
	return defaultMarshalText(c)
}

func (c *Commit) UnmarshalText(text []byte) error {
	return defaultUnmarshalText(text, c)
}

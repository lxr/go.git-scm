package object

import (
	"bytes"
	"fmt"
)

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
	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "tree", c.Tree)
	for _, parent := range c.Parent {
		fmt.Fprintln(buf, "parent", parent)
	}
	fmt.Fprintln(buf, "author", c.Author)
	fmt.Fprintln(buf, "committer", c.Committer)
	fmt.Fprintln(buf)
	buf.WriteString(c.Message)
	return buf.Bytes(), nil
}

func (c *Commit) UnmarshalText(text []byte) error {
	buf := bytes.NewBuffer(text)
	var err fmtErr
	err.Check(fmt.Fscanf(buf, "tree %s\n", &c.Tree))
	c.Parent = nil
	for {
		var parent ID
		if _, err := fmt.Fscanf(buf, "parent %s\n", &parent); err != nil {
			break
		}
		c.Parent = append(c.Parent, parent)
	}
	err.Check(fmt.Fscanf(buf, "author %s\n", &c.Author))
	err.Check(fmt.Fscanf(buf, "committer %s\n", &c.Committer))
	err.Check(fmt.Fscanf(buf, "\n"))
	c.Message = buf.String()
	return err.Err()
}

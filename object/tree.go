package object

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
)

// A filename is a string with the distinction that, when scanned, it
// is only broken on newlines and null bytes.
type filename string

func (f *filename) Scan(ss fmt.ScanState, verb rune) error {
	// BUG(lor): Filenames with newlines are not supported in trees.
	name, err := getToken(ss, "\x00\x0A/", "")
	if err != nil {
		return err
	}
	*f = filename(name)
	return nil
}

// A Tree is a mapping from filenames to tree metadata (most
// importantly, object IDs), analogous to a filesystem directory.
type Tree map[string]TreeInfo

// A TreeInfo represents the metadata Git associates with a tree entry.
type TreeInfo struct {
	Mode   TreeMode
	Object ID
}

// A TreeMode is a six-digit octal number representing a tree entry's
// mode and permission bits.  It is identical in format to a Unix file
// mode, though Git supports only a fixed subset of possible values.
// A TreeMode's function is to encode the Git object and Unix file types
// of its associated object.
type TreeMode uint32

// The recognized modes for Git tree entries.  The encoded Git object
// and Unix file types are commented to the right.
const (
	ModeTree    TreeMode = 0040000 // tree, directory
	ModeBlob    TreeMode = 0100644 // blob, file
	ModeExec    TreeMode = 0100755 // blob, file
	ModeSymlink TreeMode = 0120000 // blob, file
	ModeGitlink TreeMode = 0160000 // commit, directory
)

// Type returns the Git object type encoded by the mode.  It returns
// TypeUnknown if the mode does not have an associated type.
func (m TreeMode) Type() Type {
	switch m {
	case ModeTree:
		return TypeTree
	case ModeBlob, ModeExec, ModeSymlink:
		return TypeBlob
	case ModeGitlink:
		return TypeCommit
	default:
		return TypeUnknown
	}
}

// Names returns the names of the tree's entries in the Git order:
// ascending in C locale, with the exception that the names of sub-trees
// are sorted as if they had a trailing slash.
func (t Tree) Names() []string {
	i := 0
	names := make(sort.StringSlice, len(t))
	for name, ti := range t {
		names[i] = name
		if ti.Mode == ModeTree {
			names[i] += "/"
		}
		i++
	}
	names.Sort()
	for i, name := range names {
		names[i] = strings.TrimSuffix(name, "/")
	}
	return names
}

func (t Tree) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	for _, name := range t.Names() {
		ti := t[name]
		fmt.Fprintf(buf, "%o %s\x00", ti.Mode, name)
		buf.Write(ti.Object[:])
	}
	return prependHeader(TypeTree, buf.Bytes())
}

func (t Tree) UnmarshalBinary(data []byte) error {
	data, err := stripHeader(TypeTree, data)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(data)
	for buf.Len() > 0 {
		var ti TreeInfo
		var name filename
		if _, err := fmt.Fscanf(buf, "%o %s\x00", &ti.Mode, &name); err != nil {
			return err
		}
		if _, err := io.ReadFull(buf, ti.Object[:]); err != nil {
			return err
		}
		t[string(name)] = ti
	}
	return nil
}

func (t Tree) MarshalText() ([]byte, error) {
	buf := new(bytes.Buffer)
	for _, name := range t.Names() {
		ti := t[name]
		fmt.Fprintf(buf, "%06o %s %s\t%s\n",
			ti.Mode,
			ti.Mode.Type(),
			ti.Object,
			name,
		)
	}
	return buf.Bytes(), nil
}

func (t Tree) UnmarshalText(text []byte) error {
	buf := bytes.NewBuffer(text)
	for buf.Len() > 0 {
		var ti TreeInfo
		var objType Type
		var name filename
		_, err := fmt.Fscanf(buf, "%06o %s %s\t%s\n",
			&ti.Mode,
			&objType,
			&ti.Object,
			&name,
		)
		if err != nil {
			return err
		}
		t[string(name)] = ti
	}
	return nil
}

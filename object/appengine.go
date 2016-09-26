// +build appengine

// This file provides App Engine datastore.PropertyLoadSaver
// implementations for the Git object and data types.  This is secreted
// away behind an App Engine build constraint so that the Load and Save
// methods, which won't be relevant in the general case, won't pollute
// the documentation.

package object

import (
	"fmt"
	"strings"
	"time"

	"google.golang.org/appengine/datastore"
)

// parseProps parses a list of datastore properties into a property
// name-value map.  If a property has the Multiple flag set, its values
// will be collected into an interface{} slice.  If a property name has
// several dot-separated components, it is considered to be part of a
// sub-propertylist: properties sharing the same first name component
// have said component stripped, and are collected into a
// datalist.Property slice stored in the map under the stripped
// component.
func parseProps(props []datastore.Property) map[string]interface{} {
	m := make(map[string]interface{})
	for _, prop := range props {
		comps := strings.SplitN(prop.Name, ".", 2)
		name := comps[0]
		switch {
		case len(comps) == 2:
			prop.Name = comps[1]
			subprops, _ := m[name].([]datastore.Property)
			m[name] = append(subprops, prop)
		case prop.Multiple:
			values, _ := m[name].([]interface{})
			m[name] = append(values, prop.Value)
		default:
			m[name] = prop.Value
		}
	}
	return m
}

// prefixProps returns a copy of props with prefix and a dot prefixed
// to every property name.
func prefixProps(prefix string, props []datastore.Property) []datastore.Property {
	nprops := make([]datastore.Property, len(props))
	for i, prop := range props {
		prop.Name = prefix + "." + prop.Name
		nprops[i] = prop
	}
	return nprops
}

// panicHandler catches error-typed panics and stores them in *err,
// passing all other types along unmolested.
func panicHandler(err *error) {
	switch r := recover().(type) {
	case nil:
	case error:
		*err = r
	default:
		panic(r)
	}
}

// mustDecodeID and mustDecodeType parse s as a Git object ID and Type
// respectively, or panic trying.

func mustDecodeID(s string) ID {
	id, err := DecodeID(s)
	if err != nil {
		panic(err)
	}
	return id
}

func mustDecodeType(s string) Type {
	var t Type
	_, err := fmt.Sscan(s, &t)
	if err != nil {
		panic(err)
	}
	return t
}

// The PropertyLoadSaver implementations.

func (id *ID) Save() ([]datastore.Property, error) {
	return []datastore.Property{
		{Name: "ID", Value: id.String()},
	}, nil
}

func (id *ID) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	m := parseProps(props)
	*id = mustDecodeID(m["ID"].(string))
	return nil
}

func (s *Signature) Save() ([]datastore.Property, error) {
	_, offset := s.Date.Zone()
	return []datastore.Property{
		{Name: "Name", Value: s.Name},
		{Name: "Email", Value: s.Email},
		{Name: "Date", Value: s.Date},
		{Name: "TZ", Value: int64(offset)},
	}, nil
}

func (s *Signature) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	m := parseProps(props)
	*s = Signature{
		Name:  m["Name"].(string),
		Email: m["Email"].(string),
		Date: m["Date"].(time.Time).
			In(time.FixedZone("", int(m["TZ"].(int64)))),
	}
	return nil
}

func (t *Type) Save() ([]datastore.Property, error) {
	return []datastore.Property{
		{Name: "Type", Value: t.String()},
	}, nil
}

func (t *Type) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	m := parseProps(props)
	*t = mustDecodeType(m["Type"].(string))
	return nil
}

func (b *Blob) Save() ([]datastore.Property, error) {
	return []datastore.Property{
		{Name: "Contents", Value: []byte(*b), NoIndex: true},
	}, nil
}

func (b *Blob) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	m := parseProps(props)
	*b = Blob(m["Contents"].([]byte))
	return nil
}

func (t *Tag) Save() ([]datastore.Property, error) {
	props := []datastore.Property{
		{Name: "Object", Value: t.Object.String()},
		{Name: "Type", Value: t.Type.String()},
		{Name: "Tag", Value: t.Tag},
		{Name: "Message", Value: t.Message, NoIndex: true},
	}
	tagger, _ := t.Tagger.Save()
	return append(props, prefixProps("Tagger", tagger)...), nil
}

func (t *Tag) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	m := parseProps(props)
	*t = Tag{
		Object:  mustDecodeID(m["Object"].(string)),
		Type:    mustDecodeType(m["Type"].(string)),
		Tag:     m["Tag"].(string),
		Message: m["Message"].(string),
	}
	return t.Tagger.Load(m["Tagger"].([]datastore.Property))
}

func (c *Commit) Save() ([]datastore.Property, error) {
	props := []datastore.Property{
		{Name: "Tree", Value: c.Tree.String()},
		{Name: "Message", Value: c.Message, NoIndex: true},
	}
	for _, Parent := range c.Parent {
		props = append(props, datastore.Property{
			Name:     "Parent",
			Value:    Parent.String(),
			Multiple: true,
		})
	}
	Author, _ := c.Author.Save()
	Committer, _ := c.Committer.Save()
	props = append(props, prefixProps("Author", Author)...)
	props = append(props, prefixProps("Committer", Committer)...)
	return props, nil
}

func (c *Commit) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	m := parseProps(props)
	*c = Commit{
		Tree:    mustDecodeID(m["Tree"].(string)),
		Message: m["Message"].(string),
	}
	parents, _ := m["Parent"].([]interface{})
	for _, parent := range parents {
		c.Parent = append(c.Parent, mustDecodeID(parent.(string)))
	}
	if err := c.Author.Load(m["Author"].([]datastore.Property)); err != nil {
		return err
	}
	if err := c.Committer.Load(m["Committer"].([]datastore.Property)); err != nil {
		return err
	}
	return nil
}

func (t *Tree) Save() ([]datastore.Property, error) {
	var props []datastore.Property
	for name, ti := range *t {
		props = append(props, []datastore.Property{
			{Name: "Name", Value: name, Multiple: true},
			{Name: "Mode", Value: int64(ti.Mode), Multiple: true},
			{Name: "Object", Value: ti.Object.String(), Multiple: true},
		}...)
	}
	return props, nil
}

func (t *Tree) Load(props []datastore.Property) (err error) {
	defer panicHandler(&err)
	if *t == nil {
		*t = make(Tree)
	}
	m := parseProps(props)
	names, _ := m["Name"].([]interface{})
	modes, _ := m["Mode"].([]interface{})
	objects, _ := m["Object"].([]interface{})
	for i, name := range names {
		(*t)[name.(string)] = TreeInfo{
			Mode:   TreeMode(modes[i].(int64)),
			Object: mustDecodeID(objects[i].(string)),
		}
	}
	return nil
}

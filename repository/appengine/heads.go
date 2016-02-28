package appengine

import (
	"google.golang.org/appengine/datastore"
)

type symref string

func (s *symref) Save() ([]datastore.Property, error) {
	return []datastore.Property{
		{Name: "Target", Value: string(*s)},
	}, nil
}

func (s *symref) Load(props []datastore.Property) error {
	if len(props) != 1 || props[0].Name != "Target" {
		return datastore.ErrInvalidEntityType
	}
	target, ok := props[0].Value.(string)
	if !ok {
		return datastore.ErrInvalidEntityType
	}
	*s = symref(target)
	return nil
}

func (r *repo) headKey(name string) *datastore.Key {
	if name == "" {
		return r.head
	}
	return datastore.NewKey(r.ctx, "head", name, 0, r.head)
}

func (r *repo) GetHead(name string) (string, error) {
	var target symref
	return string(target), r.get(r.headKey(name), &target)
}

func (r *repo) SetHead(name string, target string) error {
	s := symref(target)
	return r.put(r.headKey(name), &s)
}

func (r *repo) DelHead(name string) error {
	return r.del(r.headKey(name))
}

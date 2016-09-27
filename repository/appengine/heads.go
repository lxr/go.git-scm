package appengine

import (
	"google.golang.org/appengine/datastore"
)

type head string

func (h *head) Save() ([]datastore.Property, error) {
	return []datastore.Property{
		{Name: "HEAD", Value: string(*h)},
	}, nil
}

func (h *head) Load(props []datastore.Property) error {
	if len(props) != 1 || props[0].Name != "HEAD" {
		return datastore.ErrInvalidEntityType
	}
	target, ok := props[0].Value.(string)
	if !ok {
		return datastore.ErrInvalidEntityType
	}
	*h = head(target)
	return nil
}

func (r *repo) GetHEAD() (string, error) {
	var head head
	err := r.get(r.root, &head)
	return string(head), err
}

func (r *repo) SetHEAD(name string) error {
	s := head(name)
	return r.put(r.root, &s)
}

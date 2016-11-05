package repository

import (
	"fmt"
	"strings"

	"github.com/lxr/go.git-scm/object"
)

var findRefList = []string{
	"refs/%s",
	"refs/tags/%s",
	"refs/heads/%s",
	"refs/remotes/%s",
}

// IsValidRef returns true if the argument refname is valid according
// to the git-check-ref-format(1) rules.  Interface implementations
// should use this to check that the arguments to the ref methods are
// well-formed.
func IsValidRef(name string) bool {
	return strings.HasPrefix(name, "refs/") &&
		!strings.Contains(name, "/.") &&
		!strings.Contains(name, "..") &&
		strings.IndexFunc(name, func(r rune) bool {
			return r < 0x20 ||
				r == 0x7F ||
				r == ' ' ||
				r == '~' ||
				r == '^' ||
				r == ':' ||
				r == '?' ||
				r == '['
		}) == -1 &&
		!strings.HasSuffix(name, "/") &&
		!strings.Contains(name, "//") &&
		!strings.HasSuffix(name, ".") &&
		!strings.HasSuffix(name, ".lock") &&
		!strings.Contains(name, "@{") &&
		!strings.Contains(name, `\`)
}

// FindRef disambiguates an abbreviated refname according to the
// gitrevisions(7) rules.
func FindRef(r Interface, name string) (object.ID, error) {
	for _, format := range findRefList {
		if id, err := r.GetRef(fmt.Sprintf(format, name)); err == nil {
			return id, err
		}
	}
	return object.ZeroID, ErrRefNotExist
}

// A sample App Engine application that serves a single datastore-backed
// repository over the smart HTTP protocol at / with unauthenticated
// read-write access.  Start by running ``goapp serve'' in its
// containing directory.
package main

import (
	"net/http"

	git_http "github.com/lxr/go.git-scm/protocol/http"
	"github.com/lxr/go.git-scm/repository"
	git_appengine "github.com/lxr/go.git-scm/repository/appengine"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
)

func init() {
	http.HandleFunc("/info/refs", advertiseRefs)
	http.HandleFunc("/git-upload-pack", uploadPack)
	http.HandleFunc("/git-receive-pack", receivePack)
}

func getRepository(r *http.Request) (repository.Interface, error) {
	c := appengine.NewContext(r)
	root := datastore.NewKey(c, "repo", "root", 0, nil)
	return git_appengine.InitRepository(c, root, "git:")
}

func advertiseRefs(w http.ResponseWriter, r *http.Request) {
	repo, err := getRepository(r)
	if err != nil {
		httpError(w, err)
		return
	}
	git_http.AdvertiseRefs(repo, w, r)
}

func uploadPack(w http.ResponseWriter, r *http.Request) {
	repo, err := getRepository(r)
	if err != nil {
		httpError(w, err)
		return
	}
	git_http.UploadPack(repo, w, r)
}

func receivePack(w http.ResponseWriter, r *http.Request) {
	repo, err := getRepository(r)
	if err != nil {
		httpError(w, err)
		return
	}
	git_http.ReceivePack(repo, w, r)
}

func httpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

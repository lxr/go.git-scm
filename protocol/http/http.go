// Package http implements the Git smart HTTP protocol.  See
// https://www.kernel.org/pub/software/scm/git/docs/technical/http-protocol.html
// for details.  Its functions have the same semantics as their
// namesakes in package protocol.  Errors are handled by printing them
// to the http.ResponseWriter with http.Error.
package http

// BUG(lor): This package is implemented as a thin wrapper around the
// protocol package, whose functions assume that concurrent reading and
// writing of the request and response are possible.  However, on some
// HTTP protocol stack configurations it is not possible to read from
// the request body once the response writer has been written to, which
// breaks this assumption.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/lxr/go.git-scm/pktline"
	"github.com/lxr/go.git-scm/protocol"
	"github.com/lxr/go.git-scm/repository"
)

// AdvertiseRefs is invoked using GET on
// $GIT_URL/info/refs?service=$servicename.
func AdvertiseRefs(repo repository.Interface, w http.ResponseWriter, r *http.Request) {
	service := r.FormValue("service")
	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")
	// Any error in protocol.AdvertiseRefs must be caught and
	// reported prior to the pktw prints, as they cause the HTTP
	// response to be written with a successful status code.  We
	// thus need to capture AdvertiseRefs's output in a buffer
	// and copy it out later.
	buf := new(bytes.Buffer)
	if err := protocol.AdvertiseRefs(repo, buf); err != nil {
		httpError(w, err)
		return
	}
	pktw := pktline.NewWriter(w)
	pktw.WriteLine(fmt.Sprintf("# service=%s\n", service))
	pktw.Flush() // not mentioned in the docs, but required.
	io.Copy(w, buf)
}

// BUG(lor): The canonical Git client appears to expect the server to
// maintain packfile negotiation state between POST requests when
// pulling over the smart HTTP protocol without multi_ack.  As
// protocol.UploadPack does not maintain state between calls,
// UploadPack only works with HTTP clients that understand the
// multi_ack_detailed capability.

// UploadPack is invoked using POST on $GIT_URL/git-upload-pack.
func UploadPack(repo repository.Interface, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	w.Header().Set("Cache-Control", "no-cache")
	if err := protocol.UploadPack(repo, w, r.Body); err != nil {
		// BUG(lor): As protocol.UploadPack can return errors
		// even after it has written something to its writer
		// argument, it is possible for UploadPack to fail even
		// after a 200 response has been sent.
		httpError(w, err)
		return
	}
}

// ReceivePack is invoked using POST on $GIT_URL/git-receive-pack.
func ReceivePack(repo repository.Interface, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
	w.Header().Set("Cache-Control", "no-cache")
	if err := protocol.ReceivePack(repo, w, r.Body); err != nil {
		httpError(w, err)
		return
	}
}

func httpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

# Introduction

Go.git-scm is a Golang implementation of the core functionality of the
Git distributed version control system, its data formats and transfer
protocols.  It can mostly be used to transfer Git objects and refs
between repositories; porcelain features, such as commit hooks and
submodule tracking, are not implemented.

Go.git-scm was originally written to power a Google App Engine -based
barebones Git server.  See `example/appengine` for how one might be
written.  Note that go.git-scm lacks a package for interacting with
ordinary filesystem-backed Git repositories, as doing it correctly is
quite complex thanks to the various optimizations implemented in the
reference Git client.

# License

Public domain, or CC0 if not applicable.

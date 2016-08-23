// Package protocol implements the Git packfile transfer protocol.
// See https://www.kernel.org/pub/software/scm/git/docs/technical/pack-protocol.html
// for details.
package protocol

// BUG(lor): The protocol functions rely on fmt.Fprintf printing its
// formatted output in a single Write; otherwise, pkt-line boundaries
// will be broken.

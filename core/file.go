package core

import (
	"hash"
	"net/url"
)

// File denotes a remote file object
type File interface {
	URL() *url.URL
	// -1 if resource is offline / not found
	Length() int64
	// Filename
	Name() string
	Checksum() (string, string, hash.Hash)
	Provider() Provider
}

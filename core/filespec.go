package core

import (
	"hash"
	"net/url"
)

// FileSpec is about to be deprecated and replaced by File
type FileSpec struct {
	URL      *url.URL
	ID       string
	Bundle   *Bundle
	Priority int
}

// File denotes a remote file object
type File interface {
	URL() *url.URL
	// -1 if resource is offline / not found
	Length() int64
	Filename() string
	Checksum() (string, string, hash.Hash)
}

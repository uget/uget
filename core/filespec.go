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
	Filename() string
	Length() int64
	Checksum() (string, string, hash.Hash)
	URL() *url.URL
}

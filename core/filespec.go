package core

import (
	"hash"
	"net/url"
)

type FileSpec struct {
	URL      *url.URL
	Id       string
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

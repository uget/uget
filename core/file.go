package core

import "github.com/uget/uget/core/api"

// File denotes a remote file object
type File interface {
	api.File

	// Offline returns whether this file is offline.
	//
	// If this method returns true, other method calls to this object will panic.
	// As such, this method should always be checked first.
	Offline() bool

	// LengthUnknown returns whether this file's length is known
	// e.g. HEAD request without Content-Length
	LengthUnknown() bool

	// done callback when this file is done downloading.
	// also ensures File is not implemented outside this package
	done()
}

var _ File = offlineFile{}
var _ File = onlineFile{}

type offlineFile struct {
	api.File
}

func (f offlineFile) Offline() bool {
	return true
}

// panics
func (f offlineFile) LengthUnknown() bool {
	panic("LengthKnown called on offline file")
}

func (f offlineFile) done() {
	panic("done called on offline file")
}

type onlineFile struct {
	api.File
	donecb func()
}

func (f onlineFile) done() {
	f.donecb()
}

func (f onlineFile) Offline() bool {
	return false
}

func (f onlineFile) LengthUnknown() bool {
	return f.Size() == api.FileSizeUnknown
}

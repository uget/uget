package core

import (
	"net/url"

	"github.com/uget/uget/core/api"
)

// File denotes a remote file object
type File interface {
	api.File

	// Err returns the error associated with this file.
	Err() error

	// Offline returns whether this file is offline.
	//
	// If this method returns true, other method calls to this object will panic.
	// As such, this method should always be checked first.
	Offline() bool

	// LengthUnknown returns whether this file's length is known
	// e.g. HEAD request without Content-Length.
	// panics if offline
	LengthUnknown() bool

	// OriginalURL returns the original URL that ultimately yielded this File
	OriginalURL() *url.URL

	// done callback when this file is done downloading.
	// also ensures File is not implemented outside this package
	// panics if offline
	done()
}

var _ File = onlineFile{}
var _ File = offlineFile{}
var _ File = erroredFile{}

func online(f api.File, orig *url.URL, done func()) File { return onlineFile{file{f, orig}, done} }

func offline(orig, curr *url.URL) File { return offlineFile{file{nil, orig}, curr} }

func errored(orig *url.URL, err error) File { return erroredFile{file{nil, orig}, err} }

type file struct {
	api.File
	original *url.URL
}

func (f file) OriginalURL() *url.URL { return f.original }

type onlineFile struct {
	file
	donecb func()
}

func (f onlineFile) Err() error          { return nil }
func (f onlineFile) Offline() bool       { return false }
func (f onlineFile) LengthUnknown() bool { return f.Size() == api.FileSizeUnknown }
func (f onlineFile) done()               { f.donecb() }

type offlineFile struct {
	file
	u *url.URL
}

func (f offlineFile) Err() error          { return nil }
func (f offlineFile) Offline() bool       { return true }
func (f offlineFile) LengthUnknown() bool { panic("LengthUnknown() on offline file") }
func (f offlineFile) done()               { panic("done() on offline file") }
func (f offlineFile) URL() *url.URL       { return f.u }

type erroredFile struct {
	file
	err error
}

func (f erroredFile) Err() error          { return f.err }
func (f erroredFile) Offline() bool       { panic("Offline() on errored file") }
func (f erroredFile) LengthUnknown() bool { panic("LengthUnknown() on errored file") }
func (f erroredFile) done()               { panic("done() on errored file") }
func (f erroredFile) URL() *url.URL       { return f.OriginalURL() }

package core

import (
	"net/url"

	"github.com/uget/uget/core/api"
)

// File denotes a remote file object
//
// For any given File, the order of method calls must be:
//     1. `Err()` - if this returns `nil`, continue with checking the file's availability:
//     2. `Offline()` - and if this also returns false, the file is valid and available.
// If `Err()` returns an error, `Offline()` and all non-URL methods will panic.
// Same for when `Offline()` returns `true`.
type File interface {
	api.File

	// Err returns the error associated with this file, if there is any.
	//
	// Read the note on call order in the interface description..
	Err() error

	// Offline returns whether this file is offline.
	//
	// Read the note on call order in the interface description.
	Offline() bool

	// LengthUnknown returns whether this file's length is known
	// e.g. HEAD request without Content-Length.
	LengthUnknown() bool

	// OriginalURL returns the original URL (passed to Client) that ultimately yielded this File.
	OriginalURL() *url.URL

	// done callback when this file is done downloading.
	// also ensures File is not implemented outside this package.
	done()
}

var _ File = onlineFile{}
var _ File = offlineFile{}
var _ File = erroredFile{}

func online(f api.File, orig *url.URL, done func()) File { return onlineFile{file{f, orig}, done} }

func offline(orig, curr *url.URL) File { return offlineFile{file{nil, orig}, curr} }

func errored(orig, curr *url.URL, err error) File { return erroredFile{file{nil, orig}, curr, err} }

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
	u   *url.URL
	err error
}

func (f erroredFile) Err() error          { return f.err }
func (f erroredFile) Offline() bool       { panic("Offline() on errored file") }
func (f erroredFile) LengthUnknown() bool { panic("LengthUnknown() on errored file") }
func (f erroredFile) done()               { panic("done() on errored file") }
func (f erroredFile) URL() *url.URL       { return f.u }

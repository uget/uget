package core

import (
	"io"
	"os"
	path "path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

// Download is a single download object that fetches a remote file
type Download struct {
	*emission.Emitter
	UpdateInterval time.Duration
	Reader         io.ReadCloser
	Directory      string
	File           File
	Skip           bool
}

// PassThru wraps an existing io.Reader.
//
// It simply forwards the Read() call, while displaying
// the results from individual calls to it.
type passThru struct {
	io.Reader
	total int64 // Total # of bytes transferred
}

// Read 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
func (pt *passThru) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	pt.total += int64(n)
	return n, err
}

const (
	eUpdate = iota
	eDone
	eSkip
)

// NewDownloadFrom initalizes a Download object from the given File and ReadCloser
func NewDownloadFrom(file File, r io.ReadCloser) *Download {
	return &Download{
		Emitter: emission.NewEmitter(),
		Reader:  r,
		File:    file,
	}
}

// OnUpdate runs given hook every `d.UpdateInterval` with progress information
func (d *Download) OnUpdate(f func(int64)) {
	d.On(eUpdate, f)
}

// OnDone runs given hook upon finish. Passes elapsed time and error that caused the stop, if any.
func (d *Download) OnDone(f func(time.Duration, error)) {
	d.On(eDone, f)
}

// OnSkip runs given hook when the download was skipped (due to the file already existing).
func (d *Download) OnSkip(f func()) {
	d.On(eSkip, f)
}

// Start reads the response body and copies its contents to the local file and emits events
func (d *Download) Start() {
	log.Debugf("Downloading %v", d.File.Name())
	defer d.Reader.Close()
	fi, err := os.Stat(d.Path())
	if err == nil {
		if d.Skip && fi.Size() == d.File.Length() {
			// File already exists
			log.Debugf("%v already exists... Returning", d.File.Name())
			d.Emit(eSkip)
			return
		}
	} else if !os.IsNotExist(err) {
		d.Emit(eDone, 0, err)
		return
	}
	f, err := os.Create(d.Path())
	if err != nil {
		d.Emit(eDone, 0, err)
		return
	}
	defer f.Close()
	done := make(chan error, 1)
	start := time.Now()
	reader := &passThru{Reader: d.Reader}
	go func() {
		_, err := io.Copy(f, reader)
		done <- err
	}()
	for {
		select {
		case <-time.After(d.UpdateInterval):
			d.Emit(eUpdate, reader.total)
		case err := <-done:
			d.Emit(eDone, time.Now().Sub(start), err)
			return
		}
	}
}

// Path denotes the local path that the file will be downloaded to
func (d *Download) Path() string {
	return path.Join(d.Directory, d.File.Name())
}

// Length denotes the content length
func (d *Download) Length() int64 {
	return d.File.Length()
}

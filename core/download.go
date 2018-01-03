package core

import (
	"io"
	"os"
	"path"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

const (
	eUpdate = iota
	eDone
)

// Download is an object that fetches a single remote file
type Download struct {
	*emission.Emitter
	UpdateInterval time.Duration
	Provider       Provider
	File           File
	Directory      string
}

// download initalizes a Download object from the given File and ReadCloser
func download(file File) *Download {
	return &Download{
		Emitter: emission.NewEmitter(),
		File:    file,
	}
}

func (f *Download) To(dir string) *Download {
	f.Directory = dir
	return f
}

func (f *Download) Via(p Provider) *Download {
	f.Provider = p
	return f
}

// Start reads the response body and copies its contents to the local file and emits events
func (fetch *Download) Start(r io.ReadCloser) {
	log.Debugf("Downloading %v", fetch.File.Name())
	defer r.Close()
	f, err := os.Create(fetch.Path())
	if err != nil {
		fetch.Emit(eDone, 0, err)
		return
	}
	defer f.Close()
	done := make(chan error, 1)
	start := time.Now()
	reader := &passThru{Reader: r}
	go func() {
		_, err := io.Copy(f, reader)
		done <- err
	}()
	for {
		select {
		case <-time.After(fetch.UpdateInterval):
			fetch.Emit(eUpdate, reader.total)
		case err := <-done:
			fetch.Emit(eDone, time.Now().Sub(start), err)
			return
		}
	}
}

// Path denotes the local path that the file will be downloaded to
func (f *Download) Path() string {
	return path.Join(f.Directory, f.File.Name())
}

// OnUpdate runs given hook every `f.UpdateInterval` with progress information
func (f *Download) OnUpdate(fn func(int64)) {
	f.On(eUpdate, fn)
}

// OnDone runs given hook upon finish. Passes elapsed time and error that caused the stop, if any.
func (f *Download) OnDone(fn func(time.Duration, error)) {
	f.On(eDone, fn)
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

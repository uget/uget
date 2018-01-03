package core

import (
	"io"
	"os"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

// Download is an object that fetches a single remote file
type Download struct {
	*emission.Emitter
	Provider Provider
	File     File
	file     *os.File
	reader   ReadProgress
	err      error
	errMtx   *sync.RWMutex
	done     chan struct{}
}

// Done returns true if this download is finished. False otherwise
func (g *Download) Done() bool {
	select {
	case <-g.done:
		return true
	default:
		return false
	}
}

// Download initalizes a Download object from the given File and ReadCloser
func download(file File, reader ReadProgress) *Download {
	return &Download{
		Emitter: emission.NewEmitter(),
		File:    file,
		reader:  reader,
		errMtx:  new(sync.RWMutex),
		done:    make(chan struct{}),
	}
}

// Progress returns the current progress in ints
func (g *Download) Progress() int64 {
	return g.reader.Progress()
}

// Err returns the error during this download if there was one
func (g *Download) Err() error {
	g.errMtx.RLock()
	defer g.errMtx.RUnlock()
	return g.err
}

func (g *Download) to(file *os.File) *Download {
	g.file = file
	return g
}

func (g *Download) via(p Provider) *Download {
	g.Provider = p
	return g
}

// Start reads the response body, copies its contents to the local file and emits events.
// This will append to existing files. The caller needs to make sure the file does not exist!
func (g *Download) start() {
	defer close(g.done)
	_, err := io.Copy(g.file, g.reader)
	if err != nil {
		g.errMtx.Lock()
		defer g.errMtx.Unlock()
		g.err = err
	}
	logrus.Debugf("Download#start: %v done, err: %v.", g.File.Name(), err)
}

// Progress is an object that represents a long operation that can track a progress
type Progress interface {
	Progress() int64
}

// ReadProgress is an io.ReadCloser that tracks progress
type ReadProgress interface {
	io.Reader
	Progress
}

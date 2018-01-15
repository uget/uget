package core

import (
	"context"
	"io"
	"os"

	"github.com/Sirupsen/logrus"
)

// Download is an object that fetches a single remote file
// and presents information on its progress and status
type Download struct {
	Provider Provider
	File     File
	file     *os.File
	reader   ReadProgress
	canceled bool
	cancel   context.CancelFunc
	done     chan struct{}
	err      error
}

// Done returns true if this download is finished. False otherwise
func (d *Download) Done() bool {
	select {
	case <-d.done:
		return true
	default:
		return false
	}
}

// Canceled returns whether this download was canceled.
// Panics if download is still running.
func (d *Download) Canceled() bool {
	if !d.Done() {
		panic("Called Download#Canceled() when download is still running!")
	}
	return d.canceled
}

// Err returns the error during this download if there was one.
// Panics if download is still running.
func (d *Download) Err() error {
	if !d.Done() {
		panic("Called Download#Err() before download was finished!")
	}
	return d.err
}

// Progress returns the current progress in int64
func (d *Download) Progress() int64 {
	return d.reader.Progress()
}

// Wait blocks the caller until this download is finished
func (d *Download) Wait() {
	<-d.done
}

// Waiter returns the channel which is closed on completion
func (d *Download) Waiter() <-chan struct{} {
	return d.done
}

// Stop cancels this download
func (d *Download) Stop() {
	d.cancel()
}

// Download initalizes a Download object from the given File and ReadCloser
func download(file File, reader ReadProgress) *Download {
	return &Download{
		File:   file,
		reader: reader,
		done:   make(chan struct{}),
	}
}

func (d *Download) to(file *os.File) *Download {
	d.file = file
	return d
}

func (d *Download) via(p Provider) *Download {
	d.Provider = p
	return d
}

// start reads the response body, copies its contents to the local file and emits events.
// This will append to existing files. The caller needs to make sure the file does not exist!
func (d *Download) do() {
	defer close(d.done)
	_, d.err = io.Copy(d.file, d.reader)
	if d.err == context.Canceled {
		d.err = nil
		d.canceled = true
	}
	if err := d.file.Close(); err != nil {
		logrus.Errorf("Closing file failed: %v", err)
	}
	logrus.Debugf("Download#start: %v done, err: %v.", d.File.Name(), d.err)
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

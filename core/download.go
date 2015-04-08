package core

import (
	log "github.com/cihub/seelog"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

type Download struct {
	ProgressListeners []ProgressListener
	UpdateInterval    time.Duration
	Response          *http.Response
	Directory         string
	filename          string
}

type ProgressListener struct {
	Update func(float64, float64)
	Done   func(time.Duration, error)
}

func (d *Download) update(f1 float64, f2 float64) {
	for _, x := range d.ProgressListeners {
		if x.Update != nil {
			x.Update(f1, f2)
		}
	}
}

func (d *Download) done(dur time.Duration, err error) {
	for _, x := range d.ProgressListeners {
		if x.Done != nil {
			x.Done(dur, err)
		}
	}
}

func (d *Download) Start() {
	log.Debugf("Downloading %v", d.Filename())
	f, err := os.Create(d.Path())
	if err != nil {
		d.done(0, err)
		return
	}
	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := io.Copy(f, d.Response.Body)
		done <- err
	}()
	for {
		select {
		case <-time.After(d.UpdateInterval):
			stat, err := f.Stat()
			if err == nil {
				d.update(float64(stat.Size()), float64(d.Response.ContentLength))
			}
		case err := <-done:
			d.done(time.Now().Sub(start), err)
			break
		}
	}
}

func (d *Download) AddProgressListener(listener ProgressListener) {
	d.ProgressListeners = append(d.ProgressListeners, listener)
}

func (d *Download) Filename() string {
	if d.filename == "" {
		disposition := d.Response.Header.Get("Content-Disposition")
		arr := regexp.MustCompile(`filename="(.*?)"`).FindStringSubmatch(disposition)
		if len(arr) > 1 {
			d.filename = arr[1]
		} else {
			paths := strings.Split(d.Response.Request.URL.RequestURI(), "/")
			d.filename = paths[len(paths)-1]
			if d.filename == "" {
				d.filename = "index.html"
			}
		}
	}
	return d.filename
}

func (d *Download) Path() string {
	return path.Join(d.Directory, d.Filename())
}

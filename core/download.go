package core

import (
	log "github.com/Sirupsen/logrus"
	"github.com/muja/emission"
	"io"
	"net/http"
	"os"
	path "path/filepath"
	"regexp"
	"strings"
	"time"
)

type Download struct {
	*emission.Emitter
	UpdateInterval time.Duration
	Response       *http.Response
	Directory      string
	filename       string
}

const (
	eUpdate = iota
	eDone
	eSkip
)

func NewDownloadFromResponse(r *http.Response) *Download {
	return &Download{
		Emitter:  emission.NewEmitter(),
		Response: r,
	}
}

func (d *Download) OnUpdate(f func(int64, int64)) {
	d.On(eUpdate, f)
}

func (d *Download) OnDone(f func(time.Duration, error)) {
	d.On(eDone, f)
}

func (d *Download) OnSkip(f func()) {
	d.On(eSkip, f)
}

func (d *Download) Start() {
	log.Debugf("Downloading %v", d.Filename())
	defer d.Response.Body.Close()
	fi, err := os.Stat(d.Path())
	if err == nil {
		if fi.Size() == d.Response.ContentLength {
			// File already exists
			log.Debugf("%v already exists... Returning", d.Filename())
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
	go func() {
		_, err := io.Copy(f, d.Response.Body)
		done <- err
	}()
	for {
		select {
		case <-time.After(d.UpdateInterval):
			stat, err := f.Stat()
			if err == nil {
				d.Emit(eUpdate, stat.Size(), d.Length())
			}
		case err := <-done:
			d.Emit(eDone, time.Now().Sub(start), err)
			return
		}
	}
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

func (d *Download) Length() int64 {
	return d.Response.ContentLength
}

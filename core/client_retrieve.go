package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
)

func (d *Client) workRetrieve() {
	for file := range d.ResolvedQueue.get {
		if file.Err() != nil {
			panic(fmt.Sprintf("File error in retrieve: %v", file.Err()))
		} else if file.Offline() {
			d.emit(eDeadend, file.URL())
		} else {
			d.download(file)
			file.done()
		}
	}
}

func max(ps []Provider, f func(Provider) uint) Provider {
	var max uint
	var maxP Provider
	for _, p := range ps {
		prio := f(p)
		if prio > max {
			maxP = p
			max = prio
		}
	}
	return maxP
}

// Download retrieves the given File
func (d *Client) download(file File) {
	retriever := max(d.Providers, func(p Provider) uint {
		if getter, ok := p.(Retriever); ok {
			prio := getter.CanRetrieve(file)
			logrus.Debugf("Client#download (%v): provider %v with prio %v", file.Name(), p.Name(), prio)
			return prio
		}
		return 0
	}).(Retriever)

	path := filepath.Join(d.Directory, file.Name())
	fi, err := os.Stat(path)
	headers := map[string]string{}
	if err == nil {
		logrus.Debugf("Client#download (%v): local: %v, remote: %v", file.Name(), fi.Size(), file.Size())
		if fi.Size() == file.Size() {
			if d.Skip {
				logrus.Debugf("Client#download (%v): already exists... returning", file.Name())
				d.emit(eSkip, file)
				return
			}
			logrus.Debugf("Client#download (%v): already exists... deleting", file.Name())
			err = os.Remove(path)
			if err != nil {
				d.emit(eError, file, err)
				return
			}
		} else if !d.NoContinue {
			headers["Range"] = fmt.Sprintf("bytes=%d-", fi.Size())
			logrus.Infof("Client#download (%v): +header range %s", file.Name(), headers["Range"])
		}
	} else if !os.IsNotExist(err) {
		d.emit(eError, 0, err)
		return
	}
	if !d.dryRun("fetch %s with %s provider.", file.Name(), retriever.Name()) {
		if req, err := retriever.Retrieve(file); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			ctx, cancel := context.WithCancel(req.Context())
			defer cancel()
			req = req.WithContext(ctx)
			resp, err := d.httpClient.Do(req)
			if err != nil {
				d.emit(eError, file, err)
				return
			}
			defer resp.Body.Close()
			logrus.Debugf("Client#download (%v): > %v", file.Name(), resp.Request.Header)
			logrus.Debugf("Client#download (%v): %v", file.Name(), resp.Status)
			for k, v := range resp.Header {
				logrus.Debugf("  < %v: %v", k, v)
			}
			// Disallow redirects as well -- we haven't set a redirect handler
			if !strings.HasPrefix(resp.Status, "2") {
				logrus.Errorf("Client#download (%v): %v", file.Name(), resp.Status)
				d.emit(eError, file, fmt.Errorf("status code %v", resp.Status))
				return
			}
			reader := &passThru{Reader: resp.Body}
			openFlags := os.O_WRONLY | os.O_CREATE
			if resp.StatusCode == http.StatusPartialContent {
				openFlags |= os.O_APPEND
				reader.total = fi.Size()
			} else if resp.StatusCode != http.StatusOK {
				logrus.Warnf("Client#download (%v): unknown status code %v", file.Name(), resp.StatusCode)
			}
			f, err := os.OpenFile(path, openFlags, 0644)
			if err != nil {
				d.emit(eError, file, err)
				return
			}
			download := download(file, reader).to(f).via(retriever)
			download.cancel = cancel
			d.emit(eDownload, download)
			download.do()
		} else {
			d.emit(eError, file, err)
		}
	}
	logrus.Debugf("Client#download (%v): EXIT", file.Name())
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
	atomic.AddInt64(&pt.total, int64(n))
	return n, err
}

func (pt *passThru) Progress() int64 {
	return atomic.LoadInt64(&pt.total)
}

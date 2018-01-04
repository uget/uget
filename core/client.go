package core

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

// Client manages downloads
type Client struct {
	*emission.Emitter
	queue      *queue
	jobs       int
	dryrun     bool
	client     *http.Client
	Directory  string
	Skip       bool
	NoContinue bool
}

const (
	eDownload = iota
	eDeadend
	eError
	eSkip
)

// NewClient creates a new Client with 3 workers
func NewClient() *Client {
	return NewClientWith(3)
}

// NewClientWith creates a new Client with the amount of workers provided
func NewClientWith(workers int) *Client {
	dl := &Client{
		Emitter: emission.NewEmitter(),
		queue:   newQueue(),
		jobs:    workers,
		client:  &http.Client{},
	}
	return dl
}

func (d *Client) work() {
	for j := range d.queue.get {
		d.download(j)
	}
}

// AddURLs adds a list of URLs to the download queue.
// Returns a WaitGroup for when the downloads are complete.
func (d *Client) AddURLs(urls []*url.URL) *sync.WaitGroup {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fs, err := resolveSync(urls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error while resolving: %v\n", err)
			return
		}
		d.queue.enqueue(fs, wg)
	}()
	return wg
}

// DryRun starts this downloader in dryrun mode, printing to stdout instead of downloading.
func (d *Client) DryRun() {
	d.dryrun = true
	d.Start()
}

func (d *Client) dryRun(format string, is ...interface{}) bool {
	if d.dryrun {
		fmt.Printf("Would "+format, is...)
	} else {
		capitalized := strings.ToUpper(string(format[0])) + format[1:]
		logrus.Infof(capitalized, is...)
	}
	return d.dryrun
}

// Start starts the Client asynchronously
func (d *Client) Start() {
	logrus.Debugf("Client#Start: %v workers", d.jobs)
	for _, p := range providers {
		if cfg, ok := p.(Configured); ok {
			var am *AccountManager
			if acct, ok := p.(Accountant); ok {
				am = AccountManagerFor("", acct)
			}
			cfg.Configure(&Config{am})
		}
	}
	for i := 0; i < d.jobs; i++ {
		go d.work()
	}
}

type resolveJob func() ([]File, error)

// ResolveSync resolves the URLs. Returns File specs or error that occurred
func (d *Client) ResolveSync(urls []*url.URL) ([]File, error) {
	return resolveSync(urls)
}

// Resolve asynchronously resolves the URLs. Returns file channel, error channel and worker count
// The amount of messages to the channels are guaranteed to add up to the worker count,
// i.e., every worker will either send `[]File` or `error`.
//
// As such, a `for` instruction that loops `n` times (n being the third return value) and selects
// from both channels will eventually terminate.
func (d *Client) Resolve(urls []*url.URL) (<-chan []File, <-chan error, int) {
	return resolve(urls)
}

func resolveSync(urls []*url.URL) ([]File, error) {
	fs := make([]File, 0, len(urls))
	fchan, echan, len := resolve(urls)
	for i := 0; i < len; i++ {
		select {
		case files := <-fchan:
			fs = append(fs, files...)
		case err := <-echan:
			logrus.Infof("Client#resolveSync: error received: %v", err)
			return fs, err
		}
	}
	return fs, nil
}

func resolve(urls []*url.URL) (<-chan []File, <-chan error, int) {
	logrus.Debugf("Client#resolve: %v URLs", len(urls))
	byProvider := make(map[resolver][]*url.URL)
	for _, u := range urls {
		resolver := FindProvider(func(p Provider) bool {
			if r, ok := p.(resolver); ok {
				return r.CanResolve(u)
			}
			return false
		}).(resolver)
		byProvider[resolver] = append(byProvider[resolver], u)
	}
	logrus.Debug("Client#resolve: grouped.")
	jobs := make([]resolveJob, 0, len(urls))
	for p, urls := range byProvider {
		if mr, ok := p.(MultiResolver); ok {
			us := urls
			job := func() ([]File, error) {
				fs, err := mr.Resolve(us)
				return fs, err
			}
			jobs = append(jobs, job)
		} else {
			sr := p.(SingleResolver)
			for _, url := range urls {
				u := url
				job := func() ([]File, error) {
					fs, err := sr.Resolve(u)
					return []File{fs}, err
				}
				jobs = append(jobs, job)
			}
		}
	}
	logrus.Debug("Client#resolve: jobs queued.")
	fchan := make(chan []File)
	echan := make(chan error)
	for _, job := range jobs {
		go func(job resolveJob) {
			files, err := job()
			if err != nil {
				echan <- err
			} else {
				fchan <- files
			}
		}(job)
	}
	return fchan, echan, len(jobs)
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
func (d *Client) download(j *downloadJob) {
	defer j.wg.Done()
	retriever := max(providers, func(p Provider) uint {
		if getter, ok := p.(Retriever); ok {
			prio := getter.CanRetrieve(j.file)
			logrus.Debugf("Client#download (%v): provider %v with prio %v", j.file.Name(), p.Name(), prio)
			return prio
		}
		return 0
	}).(Retriever)

	path := filepath.Join(d.Directory, j.file.Name())
	fi, err := os.Stat(path)
	headers := map[string]string{}
	if err == nil {
		logrus.Debugf("Client#download (%v): local: %v, remote: %v", j.file.Name(), fi.Size(), j.file.Size())
		if fi.Size() == j.file.Size() {
			if d.Skip {
				logrus.Debugf("Client#download (%v): already exists... returning", j.file.Name())
				d.Emit(eSkip, j.file)
				return
			}
			logrus.Debugf("Client#download (%v): already exists... deleting", j.file.Name())
			err = os.Remove(path)
			if err != nil {
				d.Emit(eError, j.file, err)
				return
			}
		} else if !d.NoContinue {
			headers["Range"] = fmt.Sprintf("bytes=%d-", fi.Size())
			logrus.Infof("Client#download (%v): +header range %s", j.file.Name(), headers["Range"])
		}
	} else if !os.IsNotExist(err) {
		d.Emit(eError, 0, err)
		return
	}
	if !d.dryRun("fetch %s with %s provider.", j.file.Name(), retriever.Name()) {
		if req, err := retriever.Retrieve(j.file); err == nil {
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err := d.client.Do(req)
			if err != nil {
				d.Emit(eError, j.file, err)
				return
			}
			defer resp.Body.Close()
			logrus.Debugf("Client#download (%v): > %v", j.file.Name(), resp.Request.Header)
			logrus.Debugf("Client#download (%v): %v", j.file.Name(), resp.Status)
			for k, v := range resp.Header {
				logrus.Debugf("  < %v: %v", k, v)
			}
			// Disallow redirects as well -- we haven't set a redirect handler
			if !strings.HasPrefix(resp.Status, "2") {
				logrus.Errorf("Client#download (%v): %v", j.file.Name(), resp.Status)
				d.Emit(eError, j.file, fmt.Errorf("status code %v", resp.Status))
				return
			}
			reader := &passThru{Reader: resp.Body}
			openFlags := os.O_WRONLY | os.O_CREATE
			if resp.StatusCode == http.StatusPartialContent {
				openFlags |= os.O_APPEND
				reader.total = fi.Size()
			} else if resp.StatusCode != http.StatusOK {
				logrus.Warnf("Client#download (%v): unknown status code %v", j.file.Name(), resp.StatusCode)
			}
			f, err := os.OpenFile(path, openFlags, 0644)
			if err != nil {
				d.Emit(eError, j.file, err)
				return
			}
			defer f.Close()
			getter := download(j.file, reader).to(f).via(retriever)
			d.Emit(eDownload, getter)
			getter.start()
		} else {
			d.Emit(eError, j.file, err)
		}
	}
	logrus.Debugf("Client#download (%v): EXIT", j.file.Name())
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

// OnDownload calls the given hook when a new Download is started. The download object is passed.
func (d *Client) OnDownload(f func(*Download)) {
	d.On(eDownload, f)
}

// OnSkip calls the given hook when a download is skipped
func (d *Client) OnSkip(f func(File)) {
	d.On(eSkip, f)
}

// OnDeadend calls the given hook when a Deadend instruction was returned by the provider.
func (d *Client) OnDeadend(f func(File)) {
	d.On(eDeadend, f)
}

// OnError calls the given hook when an error occurred in `Download`
func (d *Client) OnError(f func(File, error)) {
	d.On(eError, f)
}

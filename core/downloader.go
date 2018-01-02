package core

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

// Downloader manages downloads
type Downloader struct {
	*emission.Emitter
	queue     *queue
	jobs      int
	done      chan struct{}
	dryrun    bool
	client    *http.Client
	Directory string
	Skip      bool
}

const (
	eDownload = iota
	eDeadend
	eError
	eSkip
)

// NewDownloader creates a new Downloader with 3 workers
func NewDownloader() *Downloader {
	return NewDownloaderWith(3)
}

// NewDownloaderWith creates a new Downloader with the amount of workers provided
func NewDownloaderWith(workers int) *Downloader {
	dl := &Downloader{
		Emitter: emission.NewEmitter(),
		queue:   newQueue(),
		jobs:    workers,
		done:    make(chan struct{}, 1),
		client:  &http.Client{},
	}
	return dl
}

// Finished returns a channel that will be closed when all workers are idle.
func (d *Downloader) Finished() <-chan struct{} {
	return d.done
}

func (d *Downloader) work() {
	for j := range d.queue.get {
		d.download(j)
	}
}

// AddURLs adds a list of URLs to the download queue.
// Returns a WaitGroup for when the downloads are complete.
func (d *Downloader) AddURLs(urls []*url.URL) *sync.WaitGroup {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		fs, err := resolveSync(urls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err.Error())
			return
		}
		d.queue.enqueue(fs, wg)
	}()
	return wg
}

// DryRun makes this downloader print to stdout instead of downloading.
func (d *Downloader) DryRun() {
	d.dryrun = true
	d.Start()
}

func (d *Downloader) dryRun(format string, is ...interface{}) bool {
	if d.dryrun {
		fmt.Printf("Would "+format, is...)
	} else {
		capitalized := strings.ToUpper(string(format[0])) + format[1:]
		log.Infof(capitalized, is...)
	}
	return d.dryrun
}

// Start starts the Downloader asynchronously
func (d *Downloader) Start() {
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
func (d *Downloader) ResolveSync(urls []*url.URL) ([]File, error) {
	return resolveSync(urls)
}

// Resolve asynchronously resolves the URLs. Returns file channel, error channel and worker count
// The amount of messages to the channels are guaranteed to add up to the worker count,
// i.e., every worker will either send `[]File` or `error`.
//
// As such, a `for` instruction that loops `n` times (n being the third return value) and selects
// from both channels will eventually terminate.
func (d *Downloader) Resolve(urls []*url.URL) (<-chan []File, <-chan error, int) {
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
			return fs, err
		}
	}
	return fs, nil
}

func resolve(urls []*url.URL) (<-chan []File, <-chan error, int) {
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
func (d *Downloader) download(j *downloadJob) {
	defer j.wg.Done()
	retriever := max(providers, func(p Provider) uint {
		if getter, ok := p.(Retriever); ok {
			prio := getter.CanRetrieve(j.file)
			log.Debugf("%v: provider %v with prio %v", j.file.Name(), p.Name(), prio)
			return prio
		}
		return 0
	}).(Retriever)

	// Reverse iterate -> last provider is the default provider
	// Basic provider will always do something
	fetcher := Download(j.file).Via(retriever).To(d.Directory)
	log.Debugf("INIT Getter, path: %s", fetcher.Path())
	fi, err := os.Stat(fetcher.Path())
	if err == nil {
		log.Debugf("local: %v, remote: %v", fi.Size(), fetcher.File.Length())
		if d.Skip && fi.Size() == fetcher.File.Length() {
			log.Debugf("%v already exists... Returning", fetcher.File.Name())
			d.Emit(eSkip, fetcher)
			return
		}
	} else if !os.IsNotExist(err) {
		d.Emit(eError, 0, err)
		return
	}
	if !d.dryRun("fetch %s with %s provider.\n", j.file.Name(), retriever.Name()) {
		if req, err := retriever.Retrieve(j.file); err == nil {
			resp, err := d.client.Do(req)
			if err != nil {
				d.Emit(eError, j.file, err)
				return
			}
			d.Emit(eDownload, fetcher)
			fetcher.Start(resp.Body)
		} else {
			d.Emit(eError, j.file, err)
		}
	}
}

// OnDownload calls the given hook when a new Download is started. The download object is passed.
func (d *Downloader) OnDownload(f func(*Getter)) {
	d.On(eDownload, f)
}

// OnDeadend calls the given hook when a Deadend instruction was returned by the provider.
func (d *Downloader) OnSkip(f func(*Getter)) {
	d.On(eSkip, f)
}

// OnDeadend calls the given hook when a Deadend instruction was returned by the provider.
func (d *Downloader) OnDeadend(f func(File)) {
	d.On(eDeadend, f)
}

// OnError calls the given hook when an error occurred in `Download`
func (d *Downloader) OnError(f func(File, error)) {
	d.On(eError, f)
}

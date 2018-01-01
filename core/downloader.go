package core

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/uget/uget/core/action"
)

// Downloader manages downloads
type Downloader struct {
	*emission.Emitter
	Queue        *Queue
	Client       *http.Client
	MaxDownloads int
	done         chan struct{}
}

const (
	eDownload = iota
	eDeadend
	eError
)

// NewDownloader creates a new Downloader with 3 workers
func NewDownloader() *Downloader {
	return NewDownloaderWith(3)
}

// NewDownloaderWith creates a new Downloader with the amount of workers provided
func NewDownloaderWith(workers int) *Downloader {
	jar, _ := cookiejar.New(nil)
	dl := &Downloader{
		Emitter:      emission.NewEmitter(),
		Queue:        NewQueue(),
		Client:       &http.Client{Jar: jar},
		MaxDownloads: workers,
		done:         make(chan struct{}, 1),
	}
	for _, p := range providers {
		TryLogin(p, dl)
	}
	return dl
}

// Start starts the Downloader in the mode provided
func (d *Downloader) Start(async bool) {
	if async {
		d.StartAsync()
	} else {
		d.StartSync()
	}
}

// StartSync starts the Downloader in synchronous mode
func (d *Downloader) StartSync() {
	d.StartAsync()
	<-d.done
}

// Finished returns a channel that will be closed when all workers are idle.
func (d *Downloader) Finished() <-chan struct{} {
	return d.done
}

func (d *Downloader) work() {
	for f := range d.Queue.Pop() {
		d.Download(f)
		d.Queue.Done()
	}
}

// StartAsync starts the Downloader in asynchronous mode
func (d *Downloader) StartAsync() {
	for i := 0; i < d.MaxDownloads; i++ {
		go d.work()
	}
	go func() {
		d.Queue.Wait()
		d.Queue.Close()
		close(d.done)
	}()
}

type resolveJob func() ([]File, error)

// ResolveSync resolves the URLs. Returns File specs or error that occurred
func (d *Downloader) ResolveSync(urls []*url.URL) ([]File, error) {
	fs := make([]File, 0, len(urls))
	fchan, echan, len := d.Resolve(urls)
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

// Resolve asynchronously resolves the URLs. Returns file channel, error channel and worker count
// The amount of messages to the channels are guaranteed to add up to the worker count,
// i.e., every worker will either send `[]File` or `error`.
//
// As such, a `for` instruction that loops `n` times (n being the third return value) and selects
// from both channels will eventually terminate.
func (d *Downloader) Resolve(urls []*url.URL) (<-chan []File, <-chan error, int) {
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

// Download resolves and retrieves the given FileSpec (about to be deprecated)
func (d *Downloader) Download(fs *FileSpec) {
	log.Debugf("Downloading remote file: %v", fs.URL)
	req, _ := http.NewRequest("GET", fs.URL.String(), nil)
	resp, err := d.Client.Do(req)
	if err != nil {
		log.Errorf("Error while requesting %v: %v", fs.URL.String(), err)
		d.Emit(eError, fs, err)
		return
	}
	// Reverse iterate -> last provider is the default provider
	FindProvider(func(p Provider) bool {
		if ap, ok := p.(Getter); ok {
			a := ap.Action(resp, d)
			switch a.Value {
			case action.NEXT:
				return false
			case action.REDIRECT:
				fs2 := &FileSpec{}
				*fs2 = *fs // Copy fs to fs2
				fs2.URL = resp.Request.URL.ResolveReference(a.RedirectTo)
				log.Debugf("Got redirect instruction from %v provider. Location: %v", p.Name(), fs2.URL)
				d.Download(fs2)
			case action.GOAL:
				download := NewDownloadFromResponse(resp)
				d.Emit(eDownload, download)
				download.Start()
			case action.BUNDLE:
				log.Debugf("Got bundle instructions from %v provider. Bundle size: %v", p.Name(), len(a.URLs))
				d.Queue.AddLinks(a.URLs, fs.Priority)
			case action.DEADEND:
				d.Emit(eDeadend, fs)
				log.Debugf("Reached deadend (via %v provider).", p.Name())
			}
			return true
		}
		return false
	})
}

// OnDownload calls the given hook when a new Download is started. The download object is passed.
func (d *Downloader) OnDownload(f func(*Download)) {
	d.On(eDownload, f)
}

// OnDeadend calls the given hook when a Deadend instruction was returned by the provider.
func (d *Downloader) OnDeadend(f func(*FileSpec)) {
	d.On(eDeadend, f)
}

// OnError calls the given hook when an error occurred in `Download`
func (d *Downloader) OnError(f func(*FileSpec, error)) {
	d.On(eError, f)
}

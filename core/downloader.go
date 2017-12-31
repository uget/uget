package core

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"

	log "github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/uget/uget/core/action"
)

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

func NewDownloader() *Downloader {
	return NewDownloaderWith(3)
}

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

func (d *Downloader) Start(async bool) {
	if async {
		d.StartAsync()
	} else {
		d.StartSync()
	}
}

func (d *Downloader) StartSync() {
	d.StartAsync()
	<-d.done
}

func (d *Downloader) Finished() <-chan struct{} {
	return d.done
}

func (d *Downloader) work() {
	for f := range d.Queue.Pop() {
		d.Download(f)
		d.Queue.Done()
	}
}

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

func (d *Downloader) OnDownload(f func(*Download)) {
	d.On(eDownload, f)
}

func (d *Downloader) OnDeadend(f func(*FileSpec)) {
	d.On(eDeadend, f)
}

func (d *Downloader) OnError(f func(*FileSpec, error)) {
	d.On(eError, f)
}

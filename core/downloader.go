package core

import (
	log "github.com/cihub/seelog"
	"github.com/uget/uget/core/action"
	"net/http"
	"net/http/cookiejar"
)

type Provider interface {
	Name() string
	Login(*Downloader)
	Action(*http.Response, *Downloader) *action.Action
}

type Downloader struct {
	Queue           *Queue
	Client          *http.Client
	downloadChannel chan *Download
	MaxDownloads    int
	done            chan bool
}

var providers = []Provider{}

// Register a provider. This function is not thread-safe (yet)!
func RegisterProvider(p Provider) {
	providers = append(providers, p)
}

func NewDownloader() *Downloader {
	jar, _ := cookiejar.New(nil)
	dl := &Downloader{
		Queue:           NewQueue(),
		Client:          &http.Client{Jar: jar},
		MaxDownloads:    3,
		downloadChannel: make(chan *Download, 3),
		done:            make(chan bool, 1),
	}
	for _, p := range providers {
		p.Login(dl)
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

func (d *Downloader) Finished() <-chan bool {
	return d.done
}

func (d *Downloader) work() {
	for fs := d.Queue.Pop(); fs != nil; fs = d.Queue.Pop() {
		d.Download(fs)
	}
}

func (d *Downloader) StartAsync() {
	dones := make(chan bool, d.MaxDownloads)
	for i := 0; i < d.MaxDownloads; i++ {
		go func() {
			d.work()
			dones <- true
		}()
	}
	go func() {
		for i := 0; i < d.MaxDownloads; i++ {
			<-dones
		}
		d.done <- true
	}()
}

func (d *Downloader) Download(fs *FileSpec) {
	log.Debugf("Downloading remote file: %v", fs.URL)
	req, _ := http.NewRequest("GET", fs.URL.String(), nil)
	resp, err := d.Client.Do(req)
	if err != nil {
		log.Errorf("Error while requesting %v: %v")
		return
	}
	// Reverse iterate -> last provider is the default provider
	l := len(providers)
	for i := range providers {
		p := providers[l-1-i]
		a := p.Action(resp, d)
		switch a.Value {
		case action.NEXT:
			continue
		case action.REDIRECT:
			fs2 := &FileSpec{}
			*fs2 = *fs // Copy fs to fs2
			fs2.URL = a.RedirectTo
			log.Debugf("Got redirect instruction from %v provider. Location: %v", p.Name(), fs2.URL)
			d.Download(fs2)
		case action.GOAL:
			download := &Download{Response: resp}
			d.downloadChannel <- download
			download.Start()
		case action.BUNDLE:
			log.Debugf("Got bundle instructions from %v provider. Bundle size: %v", p.Name(), len(a.Links))
			d.Queue.AddLinks(a.Links, fs.Priority)
		case action.DEADEND:
			log.Debugf("Reached deadend (via %v provider).", p.Name())
		}
		return
	}
}

func (d *Downloader) NewDownload() <-chan *Download {
	return d.downloadChannel
}

type DefaultProvider struct{}

func (p DefaultProvider) Login(d *Downloader) {}

func (p DefaultProvider) Name() string {
	return "default"
}

func (p DefaultProvider) Action(r *http.Response, d *Downloader) *action.Action {
	if r.StatusCode != http.StatusOK {
		return action.Deadend()
	}
	// TODO: Make action dependent on content type?
	// ensure underlying body is indeed a file, and not a html page / etc.
	return action.Goal()
}

func init() {
	RegisterProvider(DefaultProvider{})
}

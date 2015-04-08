package core

import (
	log "github.com/cihub/seelog"
	"github.com/muja/uget/core/action"
	"net/http"
	"net/http/cookiejar"
)

type Provider interface {
	Login(*Downloader)
	Action(*http.Response, *Downloader) *action.Action
}

type Downloader struct {
	Queue            *Queue
	Client           *http.Client
	CurrentDownloads []*Download
	MaxDownloads     int
	channel          chan *Download
}

var providers = []Provider{}

// Register a provider. This function is not thread-safe (yet)!
func RegisterProvider(p Provider) {
	providers = append(providers, p)
}

func NewDownloader() *Downloader {
	jar, _ := cookiejar.New(nil)
	dl := &Downloader{
		Queue:            NewQueue(),
		Client:           &http.Client{Jar: jar},
		MaxDownloads:     3,
		CurrentDownloads: []*Download{},
		channel:          make(chan *Download, 3),
	}
	for _, p := range providers {
		p.Login(dl)
	}
	return dl
}

func (d *Downloader) StartAsync() {
	dones := make(chan bool, d.MaxDownloads)
	for i := 0; i < d.MaxDownloads; i++ {
		go d.work(dones)
	}
	for i := 0; i < d.MaxDownloads; i++ {
		_ = <-dones
	}
}

func (d *Downloader) Start() {
	d.StartAsync()
}

func (d *Downloader) work(done chan bool) {
	for fs := d.Queue.Pop(); fs != nil; fs = d.Queue.Pop() {
		d.StartSync(fs)
	}
	done <- true
}

func (d *Downloader) StartSync(fs *FileSpec) {
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
			d.StartSync(fs2)
		case action.GOAL:
			dl := &Download{
				Response: resp,
			}
			d.channel <- dl
			dl.Start()
		case action.CONTAINER:
			d.Queue.AddLinks(a.Links, fs.Priority)
		case action.DEADEND:
		}
		return
	}
}

type DefaultProvider struct{}

func (p DefaultProvider) Login(d *Downloader) {}

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

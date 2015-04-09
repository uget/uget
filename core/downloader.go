package core

import (
	log "github.com/cihub/seelog"
	"github.com/uget/uget/core/action"
	"net/http"
	"net/http/cookiejar"
)

type Downloader struct {
	Queue           *Queue
	Client          *http.Client
	downloadChannel chan *Download
	MaxDownloads    int
	done            chan bool
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
	ProviderWhere(func(p Provider) bool {
		a := p.Action(resp, d)
		switch a.Value {
		case action.NEXT:
			return false
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
		return true
	})
}

func (d *Downloader) NewDownload() <-chan *Download {
	return d.downloadChannel
}

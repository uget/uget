package core

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

const (
	eDownload = iota
	eDeadend
	eError
	eResolveError
	eSkip
)

// Client manages downloads
type Client struct {
	*emission.Emitter
	Directory      string
	Skip           bool
	NoContinue     bool
	Providers      Providers
	httpClient     *http.Client
	resolverQueue  *queue
	resolvers      int // number of resolver jobs
	retrieverQueue *queue
	retrievers     int // number of retriever/downloader jobs
	dryrun         bool
}

// NewClient creates a new Client with 3 retrievers and 1 resolver
func NewClient() *Client {
	return NewClientWith(3, 1)
}

// NewClientWith creates a new Client with the amount of workers provided.
func NewClientWith(retrievers, resolvers int) *Client {
	return &Client{
		Emitter:        emission.NewEmitter(),
		Providers:      RegisteredProviders(),
		resolverQueue:  newQueue(),
		resolvers:      resolvers,
		retrieverQueue: newQueue(),
		retrievers:     retrievers,
		httpClient:     &http.Client{},
	}
}

// AddURLs adds a list of URLs to the download queue.
// Returns a WaitGroup for when the downloads are complete.
func (d *Client) AddURLs(urls []*url.URL) *sync.WaitGroup {
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		defer wg.Done()
		units := d.group(urls)
		wg.Add(len(units))
		for _, unit := range units {
			wrapped := &resolveJob{d, wg, unit}
			d.resolverQueue.enqueue(wrapped)
		}
	}()
	return wg
}

// Start starts the Client asynchronously
func (d *Client) Start() {
	logrus.Debugf("Client#Start: %v workers", d.retrievers)
	for _, p := range d.Providers {
		if cfg, ok := p.(Configured); ok {
			var am *AccountManager
			if acct, ok := p.(Accountant); ok {
				am = AccountManagerFor("", acct)
			}
			cfg.Configure(&Config{am})
		}
	}
	for i := 0; i < d.resolvers; i++ {
		go d.workResolve()
	}
	for i := 0; i < d.retrievers; i++ {
		go d.workRetrieve()
	}
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

// OnResolveError calls the given hook when an error occurred during `Resolve`
func (d *Client) OnResolveError(f func(*url.URL, error)) {
	d.On(eResolveError, f)
}

package core

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
)

type event int

const (
	eDownload event = iota
	eError
	eResolve
	eDeadend
	eSkip
)

// Client manages downloads
type Client struct {
	Directory     string
	Skip          bool
	NoContinue    bool
	Providers     Providers
	Accounts      map[string][]Account
	ResolvedQueue *queue
	httpClient    *http.Client
	resolverQueue *queue
	retrievers    int // number of retriever/downloader jobs
	dryrun        bool
	emitter       *emission.Emitter
}

// NewClient creates a new Client with 3 retrievers and 1 resolver
func NewClient() *Client {
	return NewClientWith(3)
}

// NewClientWith creates a new Client with the amount of workers provided.
// If amount is 0, the Client works in resolve-only mode.
func NewClientWith(retrievers int) *Client {
	return &Client{
		emitter:       emission.NewEmitter(),
		Providers:     RegisteredProviders(),
		resolverQueue: newQueue(),
		ResolvedQueue: newQueue(),
		retrievers:    retrievers,
		httpClient:    new(http.Client),
		Accounts:      make(map[string][]Account),
	}
}

// AddURLs adds a list of URLs to the download queue.
// Returns a WaitGroup for when the downloads are complete.
func (d *Client) AddURLs(urls []*url.URL) *sync.WaitGroup {
	wg := new(sync.WaitGroup)
	wg.Add(len(urls) + 1)
	go func() {
		defer wg.Done()
		requests := make([]*request, len(urls))
		for i, u := range urls {
			requests[i] = rootRequest(u, wg, i)
		}
		d.resolverQueue.enqueueAll(requests)
	}()
	return wg
}

func (d *Client) configure() {
	for _, p := range d.Providers {
		if cfg, ok := p.(Configured); ok {
			cfg.Configure(&Config{d.Accounts[p.Name()]})
		}
	}
}

// Start starts the Client asynchronously
func (d *Client) Start() {
	logrus.Debugf("Client#Start: %v workers", d.retrievers)
	d.configure()
	go d.workResolve()
	for i := 0; i < d.retrievers; i++ {
		go d.workRetrieve()
	}
}

// Use adds an account to this client's repertoire.
// The account will be passed to Resolvers upon start.
func (d *Client) Use(acc Account) {
	pkg := reflect.ValueOf(acc).Elem().Type().PkgPath()
	prov := d.Providers.FindProvider(func(p Provider) bool {
		return reflect.ValueOf(p).Elem().Type().PkgPath() == pkg
	})
	if prov == nil {
		panic(fmt.Sprintf("No provider with package path %v in this client", pkg))
	}
	d.Accounts[prov.Name()] = append(d.Accounts[prov.Name()], acc)
}

// DryRun starts this downloader in dryrun mode, printing to stdout instead of downloading.
func (d *Client) DryRun() {
	d.dryrun = true
	d.Start()
}

// Resolve starts this Client in 'Resolve' mode, meaning there are no
// retrievers, and the wait groups will not wait for the retrievers to do their job.
func (d *Client) Resolve() {
	d.retrievers = 0
	d.Start()
}

func (d *Client) Finalize() {
	d.ResolvedQueue.Finalize()
	d.resolverQueue.Finalize()
}

func (d *Client) Stop() {
	close(d.ResolvedQueue.get)
	close(d.ResolvedQueue.getAll)
	close(d.resolverQueue.get)
	close(d.resolverQueue.getAll)
}

func (d *Client) dryRun(format string, is ...interface{}) bool {
	if d.dryrun {
		fmt.Printf("Would "+format+"\n", is...)
	} else {
		capitalized := strings.ToUpper(string(format[0])) + format[1:]
		logrus.Infof(capitalized, is...)
	}
	return d.dryrun
}

// OnDownload calls the given hook when a new Download is started. The download object is passed.
func (d *Client) OnDownload(f func(*Download)) {
	d.emitter.On(eDownload, f)
}

// OnSkip calls the given hook when a download is skipped
func (d *Client) OnSkip(f func(File)) {
	d.emitter.On(eSkip, f)
}

// OnError calls the given hook when an error occurred in `Download`
func (d *Client) OnError(f func(File, error)) {
	d.emitter.On(eError, f)
}

// OnResolve calls the given hook when a resolve job is finished.
// It passes the original URLs, the File if successful or the error if not.
func (d *Client) OnResolve(f func(*url.URL, File, error)) {
	d.emitter.On(eResolve, f)
}

// OnDeadend calls the given hook when a file is offline.
func (d *Client) OnDeadend(f func(*url.URL)) {
	d.emitter.On(eDeadend, f)
}

func (d *Client) emit(e event, is ...interface{}) {
	go d.emitter.EmitSync(e, is...)
}

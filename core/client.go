package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Sirupsen/logrus"
	"github.com/chuckpreslar/emission"
	"github.com/uget/uget/core/api"
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
	NoSkip        bool
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
func (d *Client) AddURLs(urls []*url.URL) Container {
	wg := new(sync.WaitGroup)
	container := &container{id: ContainerID(urls), wg: wg}
	wg.Add(len(urls) + 1)
	go func() {
		defer wg.Done()
		requests := make([]*request, len(urls))
		for i, u := range urls {
			requests[i] = rootRequest(u, container, i)
		}
		d.resolverQueue.enqueueAll(requests)
	}()
	return container
}

func (d *Client) configure() {
	for _, p := range d.Providers {
		if cfg, ok := p.(Configured); ok {
			cfg.Configure(&Config{Accounts: d.Accounts[p.Name()]})
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

// Finalize gracefully stops this Client
func (d *Client) Finalize() {
	d.ResolvedQueue.Finalize()
	d.resolverQueue.Finalize()
}

// Stop stops this Client immediately
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

// === RESOLVE METHODS ===

// Resolve returns meta information on the given URLs
func Resolve(urls []*url.URL) []File {
	c := NewClient()
	wg := c.AddURLs(urls)
	c.Resolve()
	wg.Wait()
	c.Finalize()
	files := make([]File, 0, len(urls))
	for file := range c.ResolvedQueue.Dequeue() {
		files = append(files, file)
	}
	return files
}

func (d *Client) workResolve() {
	for jobs := range d.resolverQueue.getAll {
		d.resolve(jobs)
	}
}

func (d *Client) resolve(jobs []*request) {
	units := d.units(jobs)
	wg := new(sync.WaitGroup)
	multis := make(chan *request)
	wg.Add(len(units))
	go func() {
		reqs := make([]*request, 0, len(units))
		for req := range multis {
			reqs = append(reqs, req)
		}
		d.resolverQueue.enqueueAll(reqs)
	}()
	go func() {
		wg.Wait()
		close(multis)
	}()
	for _, unit := range units {
		go func(unit resolveUnit) {
			defer wg.Done()
			requests := unit()
			for _, req := range requests {
				request := req.(*request)
				if request.resolved() {
					if request.file.Err() == nil && request.file.Offline() || d.retrievers == 0 {
						request.done()
					} else {
						d.emit(eResolve, request.u, request.file, nil)
					}
					d.ResolvedQueue.enqueue(request)
				} else {
					_, ablty := d.resolvability(request)
					if ablty == api.Single {
						d.resolverQueue.enqueue(request)
					} else {
						multis <- request
					}
				}
			}
		}(unit)
	}
}

type resolveUnit func() []api.Request

// returns: units, retrievable (resolved)
func (d *Client) units(requests []*request) []resolveUnit {
	single, multi := d.group(requests)
	fns := make([]resolveUnit, 0, len(single)+len(multi))
	for req, resolver := range single {
		request := req
		fns = append(fns, func() []api.Request {
			reqs, err := resolver.ResolveOne(request)
			if err != nil {
				if reqs != nil {
					panic("non-nil request on err!")
				}
				reqs = req.resolvesTo(errored(req.root().u, req.u, err)).Wrap()
			}
			return reqs
		})
	}
	for resolver, reqs := range multi {
		rs := reqs
		fns = append(fns, func() []api.Request {
			reqs, err := resolver.ResolveMany(reqs)
			if err != nil {
				if reqs != nil {
					panic("non-nil requests on err!")
				}
				reqs = make([]api.Request, len(rs))
				for i, req := range rs {
					local := req.(*request)
					reqs[i] = local.resolvesTo(errored(local.root().u, local.u, err))
				}
			}
			return reqs
		})
	}
	return fns
}

// returns: (SingleResolvable, MultiResolvable)
func (d *Client) group(rs []*request) (map[*request]SingleResolver, map[MultiResolver][]api.Request) {
	single := make(map[*request]SingleResolver)
	multi := make(map[MultiResolver][]api.Request)
	for _, r := range rs {
		if r.resolved() {
			panic("Resolved in Client#group: " + r.URL().String())
		}
		resolver, ablty := d.resolvability(r)
		if ablty == api.Single {
			sr := resolver.(SingleResolver)
			single[r] = sr
		} else {
			mr := resolver.(MultiResolver)
			multi[mr] = append(multi[mr], r)
		}
	}
	return single, multi
}

func (d *Client) resolvability(r *request) (resolver, api.Resolvability) {
	for _, p := range d.Providers {
		if resolver, ok := p.(resolver); ok {
			switch resolver.CanResolve(r.URL()) {
			case api.Single:
				return resolver, api.Single
			case api.Multi:
				return resolver, api.Multi
			}
		}
	}
	panic("unreachable")
}

// === RETRIEVE METHODS ===

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
			if !d.NoSkip {
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
			reader := &passThru{length: resp.ContentLength, Reader: resp.Body}
			openFlags := os.O_WRONLY | os.O_CREATE
			if resp.StatusCode == http.StatusPartialContent {
				openFlags |= os.O_APPEND
				reader.progress = fi.Size()
				reader.length += reader.progress
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
	progress int64 // Total # of bytes transferred
	length   int64 // content length
}

// Read 'overrides' the underlying io.Reader's Read method.
// This is the one that will be called by io.Copy(). We simply
// use it to keep track of byte counts and then forward the call.
func (pt *passThru) Read(p []byte) (int, error) {
	n, err := pt.Reader.Read(p)
	atomic.AddInt64(&pt.progress, int64(n))
	return n, err
}

func (pt *passThru) Progress() int64 {
	return atomic.LoadInt64(&pt.progress)
}

func (pt *passThru) Length() int64 {
	return atomic.LoadInt64(&pt.length)
}

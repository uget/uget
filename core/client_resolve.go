package core

import (
	"net/url"
	"sync"

	"github.com/uget/uget/core/api"
)

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

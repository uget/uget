package core

import (
	"net/url"

	"github.com/uget/uget/core/api"
)

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
	for _, unit := range units {
		go func(unit resolveUnit) {
			requests := unit()
			for _, req := range requests {
				request := req.(*request)
				if request.resolved() {
					if request.file.Err() == nil && request.file.Offline() || d.retrievers == 0 {
						request.done()
					} else {
						go d.EmitSync(eResolve, request.u, request.file, nil)
					}
					d.ResolvedQueue.enqueue(request)
				} else {
					d.resolverQueue.enqueue(request)
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
				reqs = req.resolvesTo(errored(req.u, err)).Wrap()
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
					reqs[i] = local.resolvesTo(errored(local.root().u, err))
				}
			}
			return reqs
		})
	}
	return fns
}

// returns: (SingleResolvable, MultiResolvable, Retrievable)
func (d *Client) group(rs []*request) (map[*request]SingleResolver, map[MultiResolver][]api.Request) {
	single := make(map[*request]SingleResolver)
	multi := make(map[MultiResolver][]api.Request)
	for _, r := range rs {
		if r.resolved() {
			panic("Resolved in Client#group: " + r.URL().String())
		}
	Loop:
		for _, p := range d.Providers {
			if resolver, ok := p.(resolver); ok {
				switch resolver.CanResolve(r.URL()) {
				case api.Single:
					single[r] = resolver.(SingleResolver)
					break Loop
				case api.Multi:
					mr := resolver.(MultiResolver)
					multi[mr] = append(multi[mr], r)
					break Loop
				}
			}
		}
	}
	return single, multi
}

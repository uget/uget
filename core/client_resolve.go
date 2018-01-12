package core

import (
	"github.com/uget/uget/core/api"
)

func (d *Client) workResolve() {
	for jobs := range d.resolverQueue.getAll {
		d.resolve(jobs)
	}
}

func (d *Client) resolve(jobs []*request) {
	units := d.units(jobs)
	for _, unit := range units {
		go func(unit resolveUnit) {
			requests, err := unit()
			for _, req := range requests {
				request := req.(*request)
				if err != nil {
					go d.EmitSync(eResolve, req.URL, nil, err)
					request.done()
					return
				}
				if request.resolved() {
					if request.file.Offline() || d.retrievers == 0 {
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

type resolveUnit func() ([]api.Request, error)

// returns: units, retrievable (resolved)
func (d *Client) units(requests []*request) []resolveUnit {
	single, multi := d.group(requests)
	fns := make([]resolveUnit, 0, len(single)+len(multi))
	for req, resolver := range single {
		request := req
		fns = append(fns, func() ([]api.Request, error) {
			reqs, err := resolver.ResolveOne(request)
			if reqs == nil {
				reqs = []api.Request{request}
			}
			return reqs, err
		})
	}
	for resolver, reqs := range multi {
		rs := reqs
		fns = append(fns, func() ([]api.Request, error) {
			reqs, err := resolver.ResolveMany(reqs)
			if reqs == nil {
				reqs = rs
			}
			return reqs, err
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

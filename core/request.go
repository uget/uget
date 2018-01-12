package core

import (
	"math"
	"net/url"
	"sync"

	"github.com/uget/uget/core/api"
)

type request struct {
	wg     *sync.WaitGroup
	parent *request
	u      *url.URL
	prio   float64
	file   File
}

func (r *request) URL() *url.URL {
	return r.u
}

func (r *request) Root() api.Request {
	if r.parent != nil {
		return r.parent.Root()
	}
	return r
}

func (r *request) Wrap() []api.Request {
	return []api.Request{r}
}

func (r *request) ResolvesTo(f api.File) api.Request {
	child := r.child()
	child.file = onlineFile{f, r.done}
	return child
}

func (r *request) Deadend() api.Request {
	child := r.child()
	child.file = offlineFile{}
	return child
}

func (r *request) Yields(u *url.URL) api.Request {
	child := r.child()
	child.u = u
	return child
}

// Bundles yields multiple requests form this request, e.g. if this request leads to a "folder".
//
// This method increases the priority of the child requests using `math.Nextafter`
// to maintain the (sub-)order. As even larger integers have a big float64 gap between them,
// this should not be a problem! To give an example:
// Between 100,000,000,000 and 100,000,000,001 there are still 2^16 float64 values. Particularly,
// this means that in a normal run, the 100,000,000,000th request URL would have to bundle 2^16-1
// further requests just so that the 2^16th request has (insignificant) precedence over that next
// root request.
func (r *request) Bundles(us []*url.URL) []api.Request {
	// 1 Request -> n Requests. We need to add n-1 to the WaitGroup.
	// if this URL leads to e.g. an empty folder and this method was still called (error was not,
	// returned), that means the request is done and adding -1 to wg is still correct.
	r.wg.Add(len(us) - 1)
	children := make([]api.Request, len(us))
	prio := r.prio
	for i, u := range us {
		prio = math.Nextafter(prio, math.Inf(-1))
		child := r.child()
		child.prio = prio
		child.u = u
		children[i] = child
	}
	return children
}

func (r *request) done() {
	r.wg.Done()
}

func (r *request) resolved() bool {
	return r.file != nil
}

func (r *request) child() *request {
	if r.resolved() {
		panic("child() called on resolved request")
	}
	return &request{
		parent: r,
		wg:     r.wg,
		prio:   r.prio,
		u:      r.u,
	}
}

func (r *request) precedes(other api.Request) bool {
	return r.prio > other.(*request).prio
}

// the rootRequest takes integers, the float64 part is only relevant for request#Bundles
func rootRequest(u *url.URL, wg *sync.WaitGroup, priority int) *request {
	return &request{wg: wg, u: u, prio: float64(priority)}
}

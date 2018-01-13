package core

import (
	"net/url"
	"sync"

	"github.com/uget/uget/core/api"
)

type request struct {
	wg     *sync.WaitGroup
	parent *request
	u      *url.URL
	order  int
	prio   int
	file   File
}

func (r *request) depth() int {
	if r.parent == nil {
		return 0
	}
	return r.parent.depth() + 1
}

func (r *request) level(other *request, rDepth, oDepth int) (*request, *request) {
	if rDepth > oDepth {
		return r.parent.level(other, rDepth-1, oDepth)
	} else if oDepth > rDepth {
		return r.level(other.parent, rDepth, oDepth-1)
	}
	return r, other
}

// constraint: r.depth() == other.depth()
func (r *request) compareLeveled(other *request) int {
	if r.parent == nil {
		// we are at the root on both sides
		return r.order - other.order
	}
	diff := r.parent.compareLeveled(other.parent)
	if diff == 0 {
		return r.order - other.order
	}
	return diff
}

func (r *request) precedesUnleveled(other *request) bool {
	r, other = r.level(other, r.depth(), other.depth())
	return r.compareLeveled(other) < 0
}

func (r *request) less(other *request) bool {
	return r.prio < other.prio || r.precedesUnleveled(other)
}

func (r *request) URL() *url.URL {
	return r.u
}

func (r *request) root() *request {
	if r.parent != nil {
		return r.parent.root()
	}
	return r
}

func (r *request) Wrap() []api.Request {
	return []api.Request{r}
}

func (r *request) resolvesTo(f File) api.Request {
	child := r.child()
	child.file = f
	return child
}

func (r *request) ResolvesTo(f api.File) api.Request {
	return r.resolvesTo(online(f, r.root().URL(), r.done))
}

func (r *request) Deadend(u *url.URL) api.Request {
	if u == nil {
		u = r.u
	}
	child := r.child()
	child.file = offline(r.root().u, u)
	child.u = u
	return child
}

func (r *request) Yields(u *url.URL) api.Request {
	child := r.child()
	child.u = u
	return child
}

// Bundles yields multiple requests form this request, e.g. if this request leads to a "folder".
func (r *request) Bundles(urls []*url.URL) []api.Request {
	// 1 Request -> n Requests. We need to add n-1 to the WaitGroup.
	// if this URL leads to e.g. an empty folder and this method was still called (error was not,
	// returned), that means the request is done and adding -1 to wg is still correct.
	r.wg.Add(len(urls) - 1)
	children := make([]api.Request, len(urls))
	for i, u := range urls {
		child := r.child()
		child.order = i
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
		order:  0,
		u:      r.u,
	}
}

// the rootRequest takes integers, the float64 part is only relevant for request#Bundles
func rootRequest(u *url.URL, wg *sync.WaitGroup, order int) *request {
	return &request{wg: wg, u: u, order: order}
}

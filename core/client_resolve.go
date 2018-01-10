package core

import (
	"bytes"
	"net/url"
	"sync"

	"github.com/Sirupsen/logrus"
)

// ResolveSync resolves the URLs. Returns File specs or error that occurred.
func (d *Client) ResolveSync(urls []*url.URL) []ResolveResult {
	rs := make([]ResolveResult, 0, len(urls))
	rchan := d.Resolve(urls)
	for r := range rchan {
		rs = append(rs, r)
	}
	return rs
}

// Resolve asynchronously resolves the URLs.
// Returns ResolveResult channel which will be closed upon completion.
func (d *Client) Resolve(urls []*url.URL) <-chan ResolveResult {
	logrus.Debugf("Client#Resolve: %v URLs", len(urls))
	d.configure()
	rchan := make(chan ResolveResult)
	units := d.group(urls)
	wg := new(sync.WaitGroup)
	wg.Add(len(units))
	for _, unit := range units {
		go func(unit resolveUnit) {
			defer wg.Done()
			for _, r := range unit.do() {
				rchan <- r
			}
		}(unit)
	}
	go func() {
		wg.Wait()
		close(rchan)
	}()
	return rchan
}

// ResolveResult is sent through channels from the Resolve action
type ResolveResult struct {
	Data File
	Err  error
}

type resolveUnit struct {
	urls []*url.URL
	do   func() []ResolveResult
}

type resolveJob struct {
	c       *Client
	wg      *sync.WaitGroup
	resolve resolveUnit
}

func (r *resolveJob) Identifier() string {
	s := bytes.NewBufferString("RESOLVE(")
	for _, u := range r.resolve.urls {
		s.WriteString(u.String())
	}
	s.WriteString(")")
	return s.String()
}

func (r *resolveJob) Do() {
	defer r.wg.Done()
	results := r.resolve.do()
	jobCount := 0
	for i, res := range results {
		if res.Err != nil {
			logrus.Warnf("Resolve fail: %v", res.Err)
		} else {
			jobCount++
			logrus.Debugf("Resolve success. Enqueueuing %v", res.Data.Name())
			r.c.retrieverQueue.enqueue(&retrieveJob{r.c, r.wg, res.Data})
		}
		go r.c.EmitSync(eResolve, r.resolve.urls[i], res.Data, res.Err)
	}
	r.wg.Add(jobCount)
}

func (d *Client) workResolve() {
	for j := range d.resolverQueue.get {
		j.Do()
	}
}

func (d *Client) group(urls []*url.URL) []resolveUnit {
	jobs := make([]resolveUnit, 0, len(urls))
	byProvider := make(map[string][]*url.URL)
	for _, u := range urls {
		resolver := d.Providers.FindProvider(func(p Provider) bool {
			if r, ok := p.(resolver); ok {
				return r.CanResolve(u)
			}
			return false
		})
		if mr, ok := resolver.(MultiResolver); ok {
			byProvider[mr.Name()] = append(byProvider[mr.Name()], u)
		} else {
			sr := resolver.(SingleResolver)
			singleURL := u
			singleJob := resolveUnit{
				do: func() []ResolveResult {
					f, err := sr.Resolve(singleURL)
					return []ResolveResult{ResolveResult{f, err}}
				},
				urls: []*url.URL{singleURL},
			}
			jobs = append(jobs, singleJob)
		}
	}
	for p, urls := range byProvider {
		mr := d.Providers.GetProvider(p).(MultiResolver)
		us := urls
		job := resolveUnit{
			do: func() []ResolveResult {
				fs, err := mr.Resolve(us)
				if err != nil {
					return []ResolveResult{ResolveResult{nil, err}}
				}
				rs := make([]ResolveResult, len(fs))
				for i, f := range fs {
					rs[i] = ResolveResult{f, err}
				}
				return rs
			},
			urls: us,
		}
		jobs = append(jobs, job)
	}
	return jobs
}

package core

import (
	"net/url"

	log "github.com/Sirupsen/logrus"
	// pq "github.com/oleiade/lane"
	"sync"

	"github.com/eapache/channels"
)

// Queue is a FileSpec queue
type Queue struct {
	buffer  *channels.InfiniteChannel
	channel chan *FileSpec
	wg      *sync.WaitGroup
}

// NewQueue creates a new queue
func NewQueue() *Queue {
	q := &Queue{
		buffer:  channels.NewInfiniteChannel(),
		channel: make(chan *FileSpec),
		wg:      new(sync.WaitGroup),
	}
	channels.Unwrap(q.buffer, q.channel)
	return q
}

// Pop returns the underlying file channel
func (q *Queue) Pop() <-chan *FileSpec {
	return q.channel
}

// Close the underlying buffer
func (q *Queue) Close() {
	q.buffer.Close()
}

// Wait for this queue to finish
func (q *Queue) Wait() {
	q.wg.Wait()
}

// Done denotes one job was done
func (q *Queue) Done() {
	q.wg.Done()
}

// Push a file to the job queue
func (q *Queue) Push(f *FileSpec) {
	q.wg.Add(1)
	q.buffer.In() <- f
	log.WithField("url", f.URL).Debug("added link to queue")
}

// AddLinks adds a list of URLs to the job queue
func (q *Queue) AddLinks(urls []*url.URL, prio int) []*FileSpec {
	fs := BundleFromLinks(urls)
	for _, f := range fs {
		f.Priority = prio
		q.Push(f)
	}
	return fs
}

// FileCount returns the length of the underlying buffer
func (q *Queue) FileCount() int {
	return q.buffer.Len()
}

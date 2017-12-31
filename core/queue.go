package core

import (
	"net/url"

	log "github.com/Sirupsen/logrus"
	// pq "github.com/oleiade/lane"
	"sync"

	"github.com/eapache/channels"
)

type Job func()
type Queue struct {
	buffer  *channels.InfiniteChannel
	channel chan *FileSpec
	wg      *sync.WaitGroup
}

func NewQueue() *Queue {
	q := &Queue{
		buffer:  channels.NewInfiniteChannel(),
		channel: make(chan *FileSpec),
		wg:      new(sync.WaitGroup),
	}
	channels.Unwrap(q.buffer, q.channel)
	return q
}

func (q *Queue) Pop() <-chan *FileSpec {
	return q.channel
}

func (q *Queue) Close() {
	q.buffer.Close()
}

func (q *Queue) Wait() {
	q.wg.Wait()
}

func (q *Queue) Done() {
	q.wg.Done()
}

func (q *Queue) Push(f *FileSpec) {
	q.wg.Add(1)
	q.buffer.In() <- f
	log.WithField("url", f.URL).Debug("added link to queue")
}

func (q *Queue) AddLinks(urls []*url.URL, prio int) []*FileSpec {
	fs := BundleFromLinks(urls)
	for _, f := range fs {
		f.Priority = prio
		q.Push(f)
	}
	return fs
}

func (q *Queue) FileCount() int {
	return q.buffer.Len()
}

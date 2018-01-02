package core

import (
	"sync"
	// pq "github.com/oleiade/lane"
)

type downloadJob struct {
	file File
	wg   *sync.WaitGroup
}

type queue struct {
	jobber
	buffer []*downloadJob
	get    chan *downloadJob
}

func newQueue() *queue {
	q := &queue{
		jobber{make(chan *asyncJob)},
		make([]*downloadJob, 0, 10),
		make(chan *downloadJob),
	}
	go q.dispatch()
	return q
}

func (q *queue) dispatch() {
	for {
		if q.has() {
			select {
			case q.get <- q.buffer[0]:
				q.dequeue()
			case job := <-q.jobQueue:
				job.work()
				close(job.done)
			}
		} else {
			job := <-q.jobQueue
			job.work()
			close(job.done)
		}
	}
}

func (q *queue) has() bool {
	return len(q.buffer) > 0
}

func (q *queue) enqueue(fs []File, wg *sync.WaitGroup) {
	wg.Add(1)
	q.job(func() {
		defer wg.Done()
		if len(fs) > cap(q.buffer)-len(q.buffer) {
			buf := q.buffer
			q.buffer = make([]*downloadJob, 0, len(buf)+len(fs)+10)
			copy(q.buffer, buf)
		}
		for _, f := range fs {
			wg.Add(1)
			downloadJob := &downloadJob{f, wg}
			q.buffer = append(q.buffer, downloadJob)
		}
	})
}

func (q *queue) dequeue() *downloadJob {
	defer func() { q.buffer = q.buffer[1:] }()
	return q.buffer[0]
}

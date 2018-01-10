package core

import (
	"github.com/uget/uget/utils"
)

type clientJob interface {
	Do()
}

type queue struct {
	*utils.Jobber
	buffer []clientJob
	get    chan clientJob
}

func newQueue() *queue {
	q := &queue{
		utils.NewJobber(),
		make([]clientJob, 0, 10),
		make(chan clientJob),
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
			case job := <-q.JobQueue:
				job.Work()
				close(job.Done)
			}
		} else {
			job := <-q.JobQueue
			job.Work()
			close(job.Done)
		}
	}
}

func (q *queue) has() bool {
	return len(q.buffer) > 0
}

func (q *queue) enqueue(cj clientJob) {
	q.Job(func() {
		q.buffer = append(q.buffer, cj)
	})
}

func (q *queue) dequeue() clientJob {
	defer func() { q.buffer = q.buffer[1:] }()
	return q.buffer[0]
}

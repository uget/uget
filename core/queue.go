package core

// pq "github.com/oleiade/lane"

type clientJob interface {
	Do()
}

type queue struct {
	jobber
	buffer []clientJob
	get    chan clientJob
}

func newQueue() *queue {
	q := &queue{
		jobber{make(chan *asyncJob)},
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

func (q *queue) enqueue(cj clientJob) {
	q.job(func() {
		q.buffer = append(q.buffer, cj)
	})
}

func (q *queue) dequeue() clientJob {
	defer func() { q.buffer = q.buffer[1:] }()
	return q.buffer[0]
}

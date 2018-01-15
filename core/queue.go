package core

import (
	"container/heap"

	"github.com/uget/uget/utils"
)

type Queue interface {
	Dequeue() <-chan File
	List() []File
}

var _ Queue = new(queue)

type queue struct {
	*utils.Jobber
	*pQueue
	get       chan File
	getAll    chan []*request
	finalized bool
}

func (q *queue) Dequeue() <-chan File {
	return q.get
}

func (q *queue) List() []File {
	var pq pQueue
	<-q.Job(func() {
		pq = make(pQueue, q.Len())
		copy(pq, *q.pQueue)
	})
	cjs := make([]File, pq.Len())
	i := 0
	for pq.Len() > 0 {
		cjs[i] = pq.peek().file
		heap.Pop(&pq)
		i++
	}
	return cjs
}

func (q *queue) Set(f File, prio int) {
	q.Job(func() {
		for index, item := range *q.pQueue {
			if item.file == f {
				item.prio = prio
				heap.Fix(q, index)
				break
			}
		}
	})
}

// returns whether the remove was sucessful
func (q *queue) Remove(f File) <-chan bool {
	b := make(chan bool, 1)
	q.Job(func() {
		for index, item := range *q.pQueue {
			if item.file.ID() == f.ID() {
				heap.Remove(q, index)
				b <- true
				return
			}
		}
		b <- false
	})
	return b
}

func newQueue() *queue {
	pq := make(pQueue, 0, 31)
	get := make(chan File)
	getAll := make(chan []*request)
	q := &queue{
		utils.NewJobber(),
		&pq,
		get,
		getAll,
		false,
	}
	go q.dispatch()
	return q
}

// Finalize stops this queue gracefully,
// making it close all channels once emptied.
func (q *queue) Finalize() <-chan struct{} {
	return q.Job(func() {
		q.finalized = true
	})
}

func (q *queue) enqueue(req *request) <-chan struct{} {
	return q.Job(func() {
		heap.Push(q, req)
	})
}

func (q *queue) enqueueAll(reqs []*request) <-chan struct{} {
	return q.Job(func() {
		for _, req := range reqs {
			heap.Push(q, req)
		}
	})
}

// == private methods, not to be used from outside ==

func (q *queue) dispatch() {
	for {
		if q.Len() > 0 {
			select {
			case q.getAll <- *q.pQueue:
				pq := make(pQueue, 0)
				q.pQueue = &pq
			case q.get <- q.peek().file:
				// fmt.Printf("q#pop, prio %v, url %v\n", q.peek().prio, q.peek().u)
				heap.Pop(q)
			case job := <-q.JobQueue:
				job.Work()
				close(job.Done)
			}
		} else if q.finalized {
			close(q.get)
			close(q.getAll)
			return
		} else {
			job := <-q.JobQueue
			job.Work()
			close(job.Done)
		}
	}
}

type pQueue []*request

func (pq pQueue) Len() int {
	return len(pq)
}

func (pq pQueue) Less(i, j int) bool {
	return pq[i].less(pq[j])
}

func (pq pQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *pQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*request))
}

func (pq *pQueue) Pop() interface{} {
	*pq = (*pq)[0 : len(*pq)-1]
	return nil
}

func (pq pQueue) peek() *request {
	return pq[0]
}

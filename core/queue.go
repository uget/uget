package core

import (
	"container/heap"

	"github.com/uget/uget/utils"
)

type clientJob interface {
	Do()
	Identifier() string
}

type queue struct {
	*utils.Jobber
	*pQueue
	get chan clientJob
}

func newQueue() *queue {
	pq := make(pQueue, 0, 31)
	q := &queue{
		utils.NewJobber(),
		&pq,
		make(chan clientJob),
	}
	go q.dispatch()
	return q
}

func (q *queue) enqueue(cj clientJob) <-chan struct{} {
	return q.Job(func() {
		item := &pItem{
			value:    cj,
			priority: 1,
		}
		heap.Push(q, item)
	})
}

func (q *queue) list() []clientJob {
	var pq pQueue
	<-q.Job(func() {
		pq = make(pQueue, q.Len())
		copy(pq, *q.pQueue)
	})
	cjs := make([]clientJob, pq.Len())
	i := 0
	for pq.Len() > 0 {
		cjs[i] = heap.Pop(&pq).(*pItem).value
		i++
	}
	return cjs
}

func (q *queue) set(cj clientJob, prio int) <-chan struct{} {
	return q.Job(func() {
		for index, item := range *q.pQueue {
			if item.value == cj {
				item.priority = prio
				heap.Fix(q, index)
				break
			}
		}
	})
}

// returns whether the remove was sucessful
func (q *queue) remove(cj clientJob) <-chan bool {
	b := make(chan bool, 1)
	q.Job(func() {
		for index, item := range *q.pQueue {
			if item.value == cj {
				heap.Remove(q, index)
				b <- true
				return
			}
		}
		b <- false
	})
	return b
}

// == private methods, not to be used from outside ==

func (q *queue) dispatch() {
	for {
		if q.Len() > 0 {
			select {
			case q.get <- q.peek():
				heap.Pop(q)
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

type pItem struct {
	value    clientJob
	priority int
}

type pQueue []*pItem

func (pq pQueue) Len() int {
	return len(pq)
}

func (pq pQueue) Less(i, j int) bool {
	return pq[i].priority > pq[j].priority
}

func (pq pQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *pQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*pItem)
	item.priority = -n
	*pq = append(*pq, item)
}

func (pq *pQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func (pq pQueue) peek() clientJob {
	return pq[0].value
}

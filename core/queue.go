package core

import (
	log "github.com/cihub/seelog"
	pq "github.com/oleiade/lane"
)

type Queue struct {
	pQueue *pq.PQueue
}

func NewQueue() *Queue {
	return &Queue{
		pQueue: pq.NewPQueue(pq.MAXPQ),
	}
}

func (q *Queue) Pop() *FileSpec {
	object, _ := q.pQueue.Pop()
	if object != nil {
		fs := object.(*FileSpec)
		log.Tracef("Queue.Pop: Popped file: %v", fs.URL)
		return fs
	}
	log.Tracef("Queue.Pop: Nothing popped.")
	return nil
}

func (q *Queue) AddLinks(links []string, prio int) ([]*FileSpec, error) {
	fs, err := BundleFromLinks(links)
	if err == nil {
		for _, f := range fs {
			f.Priority = prio
			log.Tracef("Added link to queue: %v", f.URL)
			q.pQueue.Push(f, prio)
		}
	}
	return fs, err
}

func (q *Queue) FileCount() int {
	return q.pQueue.Size()
}

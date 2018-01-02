package core

type asyncJob struct {
	work func()
	done chan struct{}
}

type jobber struct {
	jobQueue chan *asyncJob
}

func (j *jobber) job(f func()) <-chan struct{} {
	job := &asyncJob{
		work: f,
		done: make(chan struct{}, 1),
	}
	j.jobQueue <- job
	return job.done
}

func (j *jobber) workLoop() {
	for {
		select {
		case job := <-j.jobQueue:
			job.work()
			close(job.done)
		}
	}
}

package utils

type AsyncJob struct {
	Work func()
	Done chan struct{}
}

type Jobber struct {
	JobQueue chan *AsyncJob
}

func (j *Jobber) Job(f func()) <-chan struct{} {
	job := &AsyncJob{
		Work: f,
		Done: make(chan struct{}, 1),
	}
	j.JobQueue <- job
	return job.Done
}

func NewJobber() *Jobber {
	return &Jobber{make(chan *AsyncJob)}
}

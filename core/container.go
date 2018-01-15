package core

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"sync"
)

// Container combines URLs that were added in the same context
type Container interface {
	ID() ContainerID
	Wait()
}

type container struct {
	id ContainerID
	wg *sync.WaitGroup
}

func (c container) Wait() {
	c.wg.Wait()
}

func (c container) ID() ContainerID {
	return c.id
}

// ContainerID calculates the sha256 sum of the underlying URLs
type ContainerID []*url.URL

func (urls ContainerID) String() string {
	sum := sha256.New()
	for _, u := range urls {
		sum.Write([]byte(u.String()))
	}
	return fmt.Sprintf("%x", sum.Sum(nil))
}

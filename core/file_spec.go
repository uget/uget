package core

import (
	"net/url"
)

type FileSpec struct {
	URL      *url.URL
	Name     string
	Bundle   *Bundle
	Priority int
}

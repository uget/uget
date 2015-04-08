package core

import (
	"net/url"
)

type FileSpec struct {
	URL      *url.URL
	Bundle   *Bundle
	Priority int
}

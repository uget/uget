package core

import (
	"net/url"
)

type FileSpec struct {
	URL      *url.URL
	Id       string
	Bundle   *Bundle
	Priority int
}

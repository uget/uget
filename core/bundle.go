package core

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
)

// Bundle denotes affiliation of files
type Bundle struct {
	ID string
}

// BundleFromLinks creates a list of FileSpecs from the provided URLs
func BundleFromLinks(urls []*url.URL) []*FileSpec {
	c := &Bundle{}
	totalSum := sha256.New()
	files := make([]*FileSpec, 0, len(urls))
	for _, u := range urls {
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(u.String())))
		io.WriteString(totalSum, u.String())
		files = append(files, &FileSpec{URL: u, Bundle: c, ID: id})
	}
	c.ID = fmt.Sprintf("%x", totalSum.Sum(nil))
	return files
}

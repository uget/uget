package core

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
)

type Bundle struct {
	Id string
}

func BundleFromLinks(urls []*url.URL) []*FileSpec {
	c := &Bundle{}
	totalSum := sha256.New()
	files := make([]*FileSpec, 0, len(urls))
	for _, u := range urls {
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(u.String())))
		io.WriteString(totalSum, u.String())
		files = append(files, &FileSpec{URL: u, Bundle: c, Id: id})
	}
	c.Id = fmt.Sprintf("%x", totalSum.Sum(nil))
	return files
}

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

func BundleFromLinks(links []string) ([]*FileSpec, error) {
	c := &Bundle{}
	totalSum := sha256.New()
	files := make([]*FileSpec, 0, len(links))
	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil {
			return files, err
		}
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(link)))
		io.WriteString(totalSum, link)
		files = append(files, &FileSpec{URL: u, Bundle: c, Id: id})
	}
	c.Id = fmt.Sprintf("%x", totalSum.Sum(nil))
	return files, nil
}

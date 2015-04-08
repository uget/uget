package core

import (
	"net/url"
)

type Bundle struct{}

func BundleFromLinks(links []string) ([]*FileSpec, error) {
	c := &Bundle{}
	files := make([]*FileSpec, 0, len(links))
	for _, link := range links {
		url, err := url.Parse(link)
		if err != nil {
			return files, err
		}
		files = append(files, &FileSpec{URL: url, Bundle: c})
	}
	return files, nil
}

package cli

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/uget/uget/core"
)

func urlsFromFile(urls *[]*url.URL, f string) error {
	file, err := os.Open(f)
	if err != nil {
		log.WithField("file", f).Error("could not open")
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		u, err := url.Parse(scanner.Text())
		if err != nil {
			return err
		}
		*urls = append(*urls, u)
	}

	if err := scanner.Err(); err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func grabURLs(args []string, opts *urlArgs) []*url.URL {
	var urls []*url.URL
	if opts.Inline {
		urls = make([]*url.URL, 0, len(args))
		for i, link := range args {
			u, err := url.Parse(link)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Couldn't parse provided link %s (#%d): %s.", link, i+1, err.Error())
				return nil
			}
			if !u.IsAbs() {
				fmt.Fprintf(os.Stderr, "Provided link %s (#%d) must be an absolute URL.\n", link, i+1)
				return nil
			}
			urls = append(urls, u)
		}
	} else {
		urls = make([]*url.URL, 0, 256)
		for _, file := range args {
			err := urlsFromFile(&urls, file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading links: %v\n", err)
				return nil
			}
		}
	}
	return urls
}

func selectPProvider(arg string) core.Accountant {
	if arg == "" {
		ps := make([]string, 0)
		for _, p := range core.AllProviders() {
			if pp, ok := p.(core.Accountant); ok {
				ps = append(ps, pp.Name())
			}
		}
		i := userSelection(ps, "Choose a provider")
		if i < 0 {
			fmt.Fprintln(os.Stderr, "Invalid selection.")
			os.Exit(1)
		}
		arg = ps[i]
	}
	provider := core.GetProvider(arg).(core.Accountant)
	if provider == nil {
		fmt.Printf("No provider found for %s\n", arg)
	}
	return provider
}

func userSelection(arr []string, prompt string) int {
	for i, x := range arr {
		fmt.Printf("- %s (%v)\n", x, i+1)
	}
	i := -1
	fmt.Printf("%s: ", prompt)
	buf := make([]byte, 256)
	read, err := os.Stdin.Read(buf)
	if err != nil {
		panic(err)
	}
	str := strings.TrimSpace(string(buf[:read]))
	if len(str) > 0 {
		if _, err := fmt.Sscanf(str, "%d", &i); err != nil {
			for index, s := range arr {
				if s == str {
					i = index + 1
					break
				}
			}
		}
	}
	return i - 1
}

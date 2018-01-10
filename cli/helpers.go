package cli

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/Sirupsen/logrus"
	"github.com/uget/uget/app"
	"github.com/uget/uget/core"
)

func urlsFromFilename(urls *[]*url.URL, f string) error {
	file, err := os.Open(f)
	if err != nil {
		logrus.Errorf("helpers.urlsFromFile: could not open %v", f)
		return err
	}
	defer file.Close()
	return urlsFromFile(urls, file)
}

func urlsFromFile(urls *[]*url.URL, file *os.File) error {
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		u, err := url.Parse(scanner.Text())
		if err != nil {
			logrus.Errorf("helpers.urlsFromFile: url parse: %v", err)
			return err
		}
		*urls = append(*urls, u)
	}

	if err := scanner.Err(); err != nil {
		logrus.Errorf("helpers.urlsFromFile: scanner err: %v", err)
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
		if len(args) == 0 {
			args = []string{"-"}
		}
		urls = make([]*url.URL, 0, 64)
		for _, file := range args {
			var err error
			if file == "-" {
				if isatty.IsTerminal(os.Stdin.Fd()) {
					fmt.Println("Enter your links:")
				}
				err = urlsFromFile(&urls, os.Stdin)
			} else {
				err = urlsFromFilename(&urls, file)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading links: %v\n", err)
				return nil
			}
		}
	}
	if len(urls) == 0 {
		fmt.Fprintln(os.Stderr, "No URLs provided")
		return nil
	}
	return urls
}

func selectPProvider(arg string) core.Provider {
	if arg == "" {
		ps := make([]string, 0)
		for _, p := range core.RegisteredProviders() {
			if pp, ok := p.(core.Accountant); ok {
				ps = append(ps, pp.Name())
			}
		}
		i, err := userSelection(ps, "Choose a provider", 2)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		arg = ps[i]
	}
	provider := core.RegisteredProviders().GetProvider(arg)
	if provider == nil {
		fmt.Printf("Error: no provider named %s.\n", arg)
	}
	return provider
}

func userSelection(arr []string, prompt string, tries uint8) (int, error) {
	if len(arr) == 0 {
		return 0, fmt.Errorf("no applicable values found")
	}
	for i, x := range arr {
		fmt.Printf("- %s (%v)\n", x, i+1)
	}
	i := -1
	invalid := ""
	fmt.Printf("%s: ", prompt)
	buf := make([]byte, 256)
	read, err := os.Stdin.Read(buf)
	if err != nil {
		return 0, err
	}
	str := strings.TrimSpace(string(buf[:read]))
	if len(str) > 0 {
		for index, s := range arr {
			if s == str {
				return index, nil
			}
		}
		// Find by string failed - try to parse int
		if _, err := fmt.Sscanf(str, "%d", &i); err != nil {
			invalid = fmt.Sprintf("%s not found", str)
		} else {
			i = i - 1
			if i >= len(arr) || i < 0 {
				invalid = "index out of range"
			}
		}
	} else {
		invalid = "no input provided"
	}
	if invalid != "" {
		if tries > 1 {
			fmt.Fprintf(os.Stdout, "Invalid selection: %v!\n\n", invalid)
			return userSelection(arr, prompt, tries-1)
		}
		return 0, fmt.Errorf(invalid)
	}
	return i, nil
}

const (
	secondsPerMinute = 60
	secondsPerHour   = secondsPerMinute * 60
	secondsPerDay    = secondsPerHour * 24
	secondsPerYear   = secondsPerDay * 365
)

func prettyTime(d time.Duration) string {
	if d >= time.Hour*24*365*100 {
		return "never"
	}
	if d < time.Second {
		return "less than a second"
	}
	var buf [16]byte // longest is 99y364d23h59m59s
	w := len(buf)
	time := int(d.Seconds())
	years := time / secondsPerYear
	time %= secondsPerYear
	days := time / secondsPerDay
	time %= secondsPerDay
	hours := time / secondsPerHour
	time %= secondsPerHour
	minutes := time / secondsPerMinute
	time %= secondsPerMinute
	seconds := time % secondsPerMinute
	if seconds != 0 {
		w--
		buf[w] = 's'
		ss := strconv.Itoa(seconds)
		w -= len(ss)
		copy(buf[w:], ss)
	}
	if minutes != 0 {
		w--
		buf[w] = 'm'
		ms := strconv.Itoa(minutes)
		w -= len(ms)
		copy(buf[w:], ms)
	}
	if hours != 0 {
		w--
		buf[w] = 'h'
		hs := strconv.Itoa(hours)
		w -= len(hs)
		copy(buf[w:], hs)
	}
	if days != 0 {
		w--
		buf[w] = 'd'
		daysStr := strconv.Itoa(days)
		w -= len(daysStr)
		copy(buf[w:], daysStr)
	}
	if years != 0 {
		w--
		buf[w] = 'y'
		ys := strconv.Itoa(years)
		w -= len(ys)
		copy(buf[w:], ys)
	}
	return string(buf[w:])
}

// tryAddAccount asks for user input and stores the account in accounts file and returns `true` --
// if provider implements `Accountant` interface. Otherwise, simply `false` is returned
func tryAddAccount(p core.Provider, pr core.Prompter) error {
	acct, ok := p.(core.Accountant)
	if ok {
		if acc, err := acct.NewAccount(pr); err == nil {
			app.AccountManagerFor("", acct).AddAccount(acc)
		} else {
			return err
		}
	} else {
		return fmt.Errorf("provider is not support accounts")
	}
	return nil
}

func useAccounts(d *core.Client) {
	for _, provider := range core.RegisteredProviders() {
		if ac, ok := provider.(core.Accountant); ok {
			for _, acc := range app.AccountManagerFor("", ac).Accounts() {
				d.Use(acc)
			}
		}
	}
}

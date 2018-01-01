package core

import (
	"errors"
	"hash"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/uget/uget/core/action"
)

// Prompter asks for user input
type Prompter interface {
	Get(f []Field) map[string]string
	Error(display string)
	Success()
}

// Field defines a question to ask the user
type Field struct {
	Key       string
	Display   string
	Sensitive bool
	Value     string
}

// Provider is the base interface, other interfaces will be dynamically infered
type Provider interface {
	Name() string
}

type resolver interface {
	Provider

	// Determines whether this provider can read meta information
	// for the provided URL.
	CanResolve(*url.URL) bool
}

// MultiResolver is a provider which can resolve multiple URLs at once
type MultiResolver interface {
	resolver

	Resolve([]*url.URL) ([]File, error)
}

// SingleResolver is a provider which can only resolve URLs one by one
type SingleResolver interface {
	resolver

	Resolve(*url.URL) (File, error)
}

// Getter is a provider which can get/download specific URLs
type Getter interface {
	Provider

	Action(*http.Response, *Downloader) *action.Action
}

// Authenticator is a provider that requires or features signing in
type Authenticator interface {
	Provider
	Login(*Downloader, *AccountManager)
}

// Accountant is a provider that stores user accounts
type Accountant interface {
	Provider

	// Retrieve (existing) account with user input obtained from Prompter param.
	NewAccount(Prompter) (Account, error)

	// returns a pointer to an internal account struct
	// which will be serialized / deserialized against
	// Example: `return &AccountData{}`
	NewTemplate() Account
}

// TryLogin tries to login to the provider for each Authenticator&Accountant
func TryLogin(p Provider, d *Downloader) bool {
	if lp, ok := p.(Authenticator); ok {
		if acct, ok := lp.(Accountant); ok {
			lp.Login(d, AccountManagerFor("", acct))
		} else {
			lp.Login(d, nil)
		}
		return true
	}
	return false
}

// TryAddAccount asks for user input and stores the account in accounts file and returns `true` --
// if provider implements `Accountant` interface. Otherwise, simply `false` is returned
func TryAddAccount(p Provider, pr Prompter) bool {
	acct, ok := p.(Accountant)
	if ok {
		if acc, err := acct.NewAccount(pr); err != nil {
			AccountManagerFor("", acct).AddAccount(acc)
			pr.Success()
		} else {
			pr.Error(err.Error())
			// return false ?
		}
	}
	return ok
}

// TryTemplate creates a new account template for the given provider and returns that --
// if provider implements `Accountant` interface. Otherwise, simply `nil` is returned
func TryTemplate(p Provider) Account {
	if acct, ok := p.(Accountant); ok {
		return acct.NewTemplate()
	}
	return nil
}

var providers = []Provider{}

// RegisterProvider is not thread-safe!!!!
func RegisterProvider(p Provider) error {
	duplicate := GetProvider(p.Name())
	if duplicate != nil {
		return errors.New("Duplicate " + p.Name() + "!")
	}
	providers = append(providers, p)
	return nil
}

// AllProviders returns a list of registered providers
func AllProviders() []Provider {
	l := len(providers)
	ps := append(make([]Provider, 0, l), providers...)
	return ps
}

// GetProvider returns the provider for the given string, or `nil` if there was none.
func GetProvider(name string) Provider {
	return FindProvider(func(p Provider) bool {
		return p.Name() == name
	})
}

// FindProvider searches providers (in reverse order).
// Returns the first to satisfy the predicate
func FindProvider(f func(Provider) bool) Provider {
	l := len(providers)
	for i := range providers {
		p := providers[l-1-i]
		if f(p) {
			return p
		}
	}
	return nil
}

type defaultProvider struct{}

var _ Getter = defaultProvider{}
var _ SingleResolver = defaultProvider{}

func (p defaultProvider) Name() string {
	return "default"
}

func (p defaultProvider) Action(r *http.Response, d *Downloader) *action.Action {
	if r.StatusCode != http.StatusOK {
		return action.Deadend()
	}
	// TODO: Make action dependent on content type?
	// ensure underlying body is indeed a file, and not a html page / etc.
	return action.Goal()
}

type file struct {
	name   string
	length int64
	url    *url.URL
}

var _ File = file{}

func (f file) URL() *url.URL {
	return f.url
}

func (f file) Filename() string {
	return f.name

}

func (f file) Length() int64 {
	return f.length
}

func (f file) Checksum() (string, string, hash.Hash) {
	return "", "", nil
}

func (p defaultProvider) CanResolve(*url.URL) bool {
	return true
}

func (p defaultProvider) Resolve(u *url.URL) (File, error) {
	if !u.IsAbs() {
	}
	c := &http.Client{}
	req, _ := http.NewRequest("HEAD", u.String(), nil)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	disposition := resp.Header.Get("Content-Disposition")
	f := file{length: resp.ContentLength, url: u}
	arr := regexp.MustCompile(`filename="(.*?)"`).FindStringSubmatch(disposition)
	if len(arr) > 1 {
		f.name = arr[1]
	} else {
		paths := strings.Split(u.RequestURI(), "/")
		rawName := paths[len(paths)-1]
		name, err := url.PathUnescape(rawName)
		if err != nil {
			name = rawName
		}
		if name == "" {
			f.name = "index.html"
		} else {
			f.name = name
		}
	}
	return f, nil
}

func init() {
	RegisterProvider(defaultProvider{})
}

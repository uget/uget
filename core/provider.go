package core

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/uget/uget/core/action"
)

type Prompter interface {
	Get(f []Field) map[string]string
	Error(display string)
	Success()
}

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

// Resolver is a provider which can resolve specific URLs
type Resolver interface {
	Provider

	// Determines whether this provider can read meta information
	// for the provided URL.
	CanResolve(*url.URL) bool

	// Resolve the given URLs.
	Resolve([]*url.URL) ([]File, error)
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

func TryLogin(p Provider, d *Downloader) bool {
	if lp, ok := p.(Authenticator); ok {
		if pp, ok := lp.(Accountant); ok {
			lp.Login(d, AccountManagerFor("", pp))
		} else {
			lp.Login(d, nil)
		}
		return true
	}
	return false
}

func TryAddAccount(p Provider, pr Prompter) bool {
	pp, ok := p.(Accountant)
	if ok {
		if acc, err := pp.NewAccount(pr); err != nil {
			AccountManagerFor("", pp).AddAccount(acc)
			pr.Success()
		} else {
			pr.Error(err.Error())
			return ok
		}
	}
	return ok
}

func TryTemplate(p Provider) (interface{}, bool) {
	if lp, ok := p.(Accountant); ok {
		return lp.NewTemplate(), true
	}
	return nil, false
}

var providers = []Provider{}

// Register a provider. This function is not thread-safe!
func RegisterProvider(p Provider) error {
	duplicate := GetProvider(p.Name())
	if duplicate != nil {
		return errors.New("Duplicate " + p.Name() + "!")
	}
	providers = append(providers, p)
	return nil
}

func AllProviders() []Provider {
	l := len(providers)
	ps := append(make([]Provider, 0, l), providers...)
	return ps
}

func GetProvider(name string) Provider {
	return FindProvider(func(p Provider) bool {
		return p.Name() == name
	})
}

// Iterate over provider and take the first.
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

type DefaultProvider struct{}

func (p DefaultProvider) Name() string {
	return "default"
}

func (p DefaultProvider) Action(r *http.Response, d *Downloader) *action.Action {
	if r.StatusCode != http.StatusOK {
		return action.Deadend()
	}
	// TODO: Make action dependent on content type?
	// ensure underlying body is indeed a file, and not a html page / etc.
	return action.Goal()
}

func init() {
	RegisterProvider(DefaultProvider{})
}

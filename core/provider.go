package core

import (
	"errors"
	"net/http"
	"net/url"

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
		if acc, err := acct.NewAccount(pr); err == nil {
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

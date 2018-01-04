package core

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
)

// Prompter asks for user input
type Prompter interface {
	Get(f []Field) (map[string]string, error)
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

// Config object
type Config struct {
	AccountManager *AccountManager
}

// Configured are providers that require some kind of configuration/initialization
type Configured interface {
	Provider

	// Configure this provider. Can be called multiple times but never concurrently
	Configure(*Config)
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

// Retriever is a provider which can download specific URLs
type Retriever interface {
	Provider

	// returns the Request object that will lead to the file
	Retrieve(File) (*http.Request, error)

	// Determines whether this provider can fetch the resource
	// pointed to by the given URL.
	//
	// Returns:
	//   - 0, if this provider cannot operate on the URL.
	//   - > 0, if this provider is suitable for handling the URL.
	//     A higher number denotes a higher suitability (i.e., basic provider will always return 1)
	//     Providers should take remaining traffic etc. into account.
	CanRetrieve(File) uint
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

// TryAddAccount asks for user input and stores the account in accounts file and returns `true` --
// if provider implements `Accountant` interface. Otherwise, simply `false` is returned
func TryAddAccount(p Provider, pr Prompter) error {
	acct, ok := p.(Accountant)
	if ok {
		if acc, err := acct.NewAccount(pr); err == nil {
			AccountManagerFor("", acct).AddAccount(acc)
		} else {
			return err
		}
	} else {
		return fmt.Errorf("provider is not support accounts")
	}
	return nil
}

// TryTemplate creates a new account template for the given provider and returns that --
// if provider implements `Accountant` interface. Otherwise, simply `nil` is returned
func TryTemplate(p Provider) Account {
	if acct, ok := p.(Accountant); ok {
		return acct.NewTemplate()
	}
	return nil
}

// Providers represents an array of providers with some utility functions
type Providers []Provider

var globalProviders = Providers{}

// RegisterProvider is not thread-safe!!!!
func RegisterProvider(p Provider) error {
	duplicate := globalProviders.GetProvider(p.Name())
	if duplicate != nil {
		return errors.New("Duplicate " + p.Name() + "!")
	}
	globalProviders = append(globalProviders, p)
	return nil
}

// RegisteredProviders returns a list of registered providers
func RegisteredProviders() Providers {
	l := len(globalProviders)
	ps := make([]Provider, l)
	for i, p := range globalProviders {
		p2v := reflect.New(reflect.Indirect(reflect.ValueOf(p)).Type())
		ps[i] = p2v.Interface().(Provider)
	}
	return ps
}

// GetProvider returns the provider for the given string, or `nil` if there was none.
func (ps Providers) GetProvider(name string) Provider {
	return ps.FindProvider(func(p Provider) bool {
		return p.Name() == name
	})
}

// FindProvider searches providers (in reverse order).
// Returns the first to satisfy the predicate
func (ps Providers) FindProvider(f func(Provider) bool) Provider {
	l := len(ps)
	for i := range ps {
		p := ps[l-1-i]
		if f(p) {
			return p
		}
	}
	return nil
}

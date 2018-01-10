package core

import (
	"errors"
	"reflect"
)

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

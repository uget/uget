package core

import (
	"errors"
	"github.com/uget/uget/core/action"
	"net/http"
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

type Provider interface {
	Name() string
	Action(*http.Response, *Downloader) *action.Action
}

type LoginProvider interface {
	Provider
	Login(*Downloader, *AccountManager)
}

type PersistentProvider interface {
	Provider

	NewAccount(Prompter) (string, interface{}, error)

	// returns a pointer to an internal account struct
	// which will be serialized / deserialized against
	// Example: `return &AccountData{}`
	NewTemplate() interface{}
}

func TryLogin(p Provider, d *Downloader) bool {
	if lp, ok := p.(LoginProvider); ok {
		if pp, ok := lp.(PersistentProvider); ok {
			lp.Login(d, AccountManagerFor("", pp))
		} else {
			lp.Login(d, nil)
		}
		return true
	}
	return false
}

func TryAddAccount(p Provider, pr Prompter) bool {
	pp, ok := p.(PersistentProvider)
	if ok {
		if id, acc, err := pp.NewAccount(pr); err != nil {
			AccountManagerFor("", pp).AddAccount(id, acc)
			pr.Success()
		} else {
			pr.Error(err.Error())
			return ok
		}
	}
	return ok
}

func TryTemplate(p Provider) (interface{}, bool) {
	if lp, ok := p.(PersistentProvider); ok {
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

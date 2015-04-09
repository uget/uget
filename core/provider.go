package core

import (
	"errors"
	"github.com/uget/uget/core/action"
	"net/http"
)

type Prompter interface {
	Define(f *Field)
	Get() map[string]string
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
	Login(*Downloader)
	Action(*http.Response, *Downloader) *action.Action
	AddAccount(Prompter)
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

func GetProvider(name string) Provider {
	return ProviderWhere(func(p Provider) bool {
		return p.Name() == name
	})
}

// Iterate over provider and take the first.
func ProviderWhere(f func(Provider) bool) Provider {
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

func (p DefaultProvider) Login(d *Downloader) {}

func (p DefaultProvider) Name() string {
	return "default"
}

func (p DefaultProvider) AddAccount(pr Prompter) {}

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

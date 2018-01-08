package core

import (
	"net/url"

	"github.com/uget/uget/core/api"
)

const FileSizeUnknown = api.FileSizeUnknown
const FileSizeOffline = api.FileSizeOffline

// Account represents a persistent record on a provider (useful e.g. to access restricted files)
type Account = api.Account

// File denotes a remote file object
type File = api.File

// Prompter asks for user input
type Prompter = api.Prompter

// Field defines a question to ask the user
type Field = api.Field

// Provider is the base interface, other interfaces will be dynamically infered
type Provider = api.Provider

// Config object
type Config = api.Config

// Configured are providers that require some kind of configuration/initialization
type Configured = api.Configured

// not DRY -- we really don't want to export this in either package.
type resolver interface {
	Provider

	// Determines whether this provider can read meta information
	// for the provided URL.
	CanResolve(*url.URL) bool
}

// MultiResolver is a provider which can resolve multiple URLs at once
type MultiResolver = api.MultiResolver

// SingleResolver is a provider which can only resolve URLs one by one
type SingleResolver = api.SingleResolver

// Retriever is a provider which can download specific URLs
type Retriever = api.Retriever

// Accountant is a provider that stores user accounts
type Accountant = api.Accountant

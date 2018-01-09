package api

import (
	"hash"
	"net/http"
	"net/url"
)

// FileSizeOffline (returned by File#Size) denotes a file is offline
const FileSizeOffline = -1

// FileSizeUnknown (returned by File#Size) denotes a file's size is unknown
// e.g. HEAD request without Content-Length
const FileSizeUnknown = -2

// Account represents a persistent record on a provider (useful e.g. to access restricted files)
type Account interface {
	// Returns a unique identifier for this account.
	// This will often be the username or e-mail.
	ID() string
}

// File denotes a remote file object
type File interface {
	URL() *url.URL
	// -1 if resource is offline / not found
	Size() int64
	// Filename
	Name() string
	Checksum() (string, string, hash.Hash)
	Provider() Provider
}

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
	Accounts []Account
}

// Configured are providers that require some kind of configuration/initialization
type Configured interface {
	Provider

	// Configure this provider. Can be called multiple times but never concurrently
	Configure(*Config)
}

// not DRY -- we really don't want to export this in either package.
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

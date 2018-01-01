package action

import (
	"net/url"
)

// Value enum
type Value int

// Action to be done
type Action struct {
	Value      Value
	RedirectTo *url.URL
	URLs       []*url.URL
}

const (
	// GOAL means: We have reached our goal. The underlying response is the requested file.
	GOAL Value = iota
	// NEXT means: This provider does not know how to handle this response.
	NEXT
	// REDIRECT means: To get to the file, follow the link provided in this action object.
	REDIRECT
	// DEADEND means: We have reached a dead end. The file is either gone or was never there.
	DEADEND
	// BUNDLE means: The underlying response contains a subset of files.
	BUNDLE
)

// Goal - see GOAL
func Goal() *Action {
	return &Action{Value: GOAL}
}

// Next - see NEXT
func Next() *Action {
	return &Action{Value: NEXT}
}

// Redirect - see REDIRECT
func Redirect(u *url.URL) *Action {
	return &Action{Value: REDIRECT, RedirectTo: u}
}

// Deadend - see DEADEND
func Deadend() *Action {
	return &Action{Value: DEADEND}
}

// Bundle - see BUNDLE
func Bundle(urls []*url.URL) *Action {
	return &Action{Value: BUNDLE, URLs: urls}
}

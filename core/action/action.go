package action

import (
	"net/url"
)

type ActionValue int

type Action struct {
	Value      ActionValue
	RedirectTo *url.URL
	Links      []string
}

const (
	// We have reached our goal. The underlying response is the requested file.
	GOAL ActionValue = iota
	// This provider does not know how to handle this response.
	NEXT
	// To get to the file, follow the link provided in this action object.
	REDIRECT
	// We have reached a dead end. The file is either gone or was never there.
	DEADEND
	// The underlying response contains a subset of files.
	BUNDLE
)

func Goal() *Action {
	return &Action{Value: GOAL}
}

func Next() *Action {
	return &Action{Value: NEXT}
}

func Redirect(u *url.URL) *Action {
	return &Action{Value: REDIRECT, RedirectTo: u}
}

func Deadend() *Action {
	return &Action{Value: DEADEND}
}

func Bundle(links []string) *Action {
	return &Action{Value: BUNDLE, Links: links}
}

package gqlclient

import (
	"encoding/json"
)

// ErrorLocation describes an error location in a GraphQL document.
//
// Line and column numbers start from 1.
type ErrorLocation struct {
	Line, Column int
}

// Error is a GraphQL error.
type Error struct {
	Message    string
	Locations  []ErrorLocation
	Path       []interface{}
	Extensions json.RawMessage
}

func (err *Error) Error() string {
	return "gqlclient: server failure: " + err.Message
}

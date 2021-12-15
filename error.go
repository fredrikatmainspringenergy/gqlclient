package gqlclient

import (
	"encoding/json"
)

type ErrorLocation struct {
	Line, Column int
}

type Error struct {
	Message    string
	Locations  []ErrorLocation
	Path       []interface{}
	Extensions json.RawMessage
}

func (err *Error) Error() string {
	return "gqlclient: server failure: " + err.Message
}

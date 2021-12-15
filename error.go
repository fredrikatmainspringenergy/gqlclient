package gqlclient

type Error struct {
	Message string
}

func (err *Error) Error() string {
	return "gqlclient: server failure: " + err.Message
}

package auth

import "net/http"

type AccessLevel int

const (
	NoAccess AccessLevel = iota
	ReadOnly
	ReadWrite
)

type NoneAuthorizer struct {
}

func (*NoneAuthorizer) GetProjectAccessLevel(*http.Request, string, string) (AccessLevel, error) {
	return ReadWrite, nil
}

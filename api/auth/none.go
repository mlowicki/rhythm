package auth

import "net/http"

type NoneAuthorizer struct {
	BaseURL string
}

func (*NoneAuthorizer) GetProjectAccessLevel(*http.Request, string, string) (AccessLevel, error) {
	return ReadWrite, nil
}

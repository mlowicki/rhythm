package auth

import "net/http"

// AccessLevel defines type of access.
type AccessLevel int

const (
	// NoAccess means on access at all (read or write).
	NoAccess AccessLevel = iota
	// ReadOnly means access only for reading.
	ReadOnly
	// ReadWrite means access both for reading and writing.
	ReadWrite
)

// NoneAuthorizer implements dummy authorizer which always grants read and write access.
type NoneAuthorizer struct {
}

// GetProjectAccessLevel returns type of access to project for request sent by client.
func (*NoneAuthorizer) GetProjectAccessLevel(*http.Request, string, string) (AccessLevel, error) {
	return ReadWrite, nil
}

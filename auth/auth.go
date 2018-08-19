package auth

type AccessLevel int

const (
	NoAccess AccessLevel = iota
	ReadOnly
	ReadWrite
)

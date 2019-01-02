package tls

import (
	"crypto/x509"
	"errors"
	"io/ioutil"
)

// BuildCertPool returns set of certificates stored under `path`.
func BuildCertPool(path string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	certs, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if ok := pool.AppendCertsFromPEM(certs); !ok {
		return nil, errors.New("No certs appended")
	}
	return pool, nil
}

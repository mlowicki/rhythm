package secrets

import (
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/secrets/vault"
	log "github.com/sirupsen/logrus"
)

type secrets interface {
	Read(string) (string, error)
}

type None struct{}

func (*None) Read(path string) (string, error) {
	return path, nil
}

func New(c *conf.Secrets) secrets {
	var backend secrets
	switch c.Backend {
	case conf.SecretsBackendVault:
		var err error
		backend, err = vault.NewClient(&c.Vault)
		if err != nil {
			log.Fatal(err)
		}
	case conf.SecretsBackendNone:
		backend = &None{}
	default:
		log.Fatalf("Unknown secrets backend: %s", c.Backend)
	}
	log.Printf("Secrets backend: %s", c.Backend)
	return backend
}

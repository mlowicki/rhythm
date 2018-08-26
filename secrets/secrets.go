package secrets

import (
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/secrets/vault"
	log "github.com/sirupsen/logrus"
)

type secrets interface {
	Read(string) (string, error)
}

func New(c *conf.Secrets) secrets {
	if c.Backend != conf.SecretsBackendVault {
		log.Fatalf("Unknown backend: %s", c.Backend)
	}
	log.Printf("Backend: %s", c.Backend)
	cli, err := vault.NewClient(&c.Vault)
	if err != nil {
		log.Fatal(err)
	}
	return cli
}

package secrets

import (
	"log"
	"net/url"

	vault "github.com/hashicorp/vault/api"
	"github.com/mlowicki/rhythm/conf"
)

func New(c *conf.Secrets) *vault.Client {
	if c.Type != conf.SecretsTypeVault {
		log.Fatalf("Unknown secrets backend type: %s\n", c.Type)
	}
	log.Printf("Secrets backend type: %s\n", c.Type)
	url, err := url.Parse(c.Vault.Address)
	if err != nil {
		log.Fatalf("Error parsing Vault address: %s\n", err)
	}
	if url.Scheme != "https" {
		log.Printf("Vault address uses HTTP scheme which is insecure. It's recommented to use HTTPS instead.")
	}
	cli, err := vault.NewClient(&vault.Config{
		Address: c.Vault.Address,
		Timeout: c.Vault.Timeout,
	})
	if err != nil {
		log.Fatalf("Error creating Vault client: %s\n", err)
	}
	cli.SetToken(c.Vault.Token)
	return cli
}

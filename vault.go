package main

import (
	"fmt"
	"log"
	"net/url"

	vault "github.com/hashicorp/vault/api"
	"github.com/mlowicki/rhythm/conf"
)

func getVaultClient(c *conf.Vault) (*vault.Client, error) {
	url, err := url.Parse(c.Address)
	if err != nil {
		return nil, fmt.Errorf("Error parsing Vault address: %s\n", err)
	}
	if url.Scheme != "https" {
		log.Printf("Vault address uses HTTP scheme which is insecure. It's recommented to use HTTPS instead.")
	}
	vaultC, err := vault.NewClient(&vault.Config{
		Address: c.Address,
		Timeout: c.Timeout,
	})
	if err != nil {
		return nil, err
	}
	vaultC.SetToken(c.Token)
	return vaultC, nil
}

package main

import (
	vault "github.com/hashicorp/vault/api"
	"github.com/mlowicki/rhythm/conf"
)

func getVaultClient(c *conf.Vault) (*vault.Client, error) {
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

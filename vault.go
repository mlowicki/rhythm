package main

import (
	vault "github.com/hashicorp/vault/api"
)

func getVaultClient(conf *ConfigVault) (*vault.Client, error) {
	vaultC, err := vault.NewClient(&vault.Config{
		Address: conf.Address,
		Timeout: conf.Timeout,
	})
	if err != nil {
		return nil, err
	}
	vaultC.SetToken(conf.Token)
	return vaultC, nil
}

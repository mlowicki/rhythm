package vault

import (
	"errors"
	"net/url"

	vault "github.com/hashicorp/vault/api"
	"github.com/mlowicki/rhythm/conf"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	c *vault.Client
}

func (c *Client) Read(path string) (string, error) {
	secret, err := c.c.Logical().Read(path)
	if err != nil {
		return "", err
	}
	if secret == nil {
		return "", errors.New("secret not found")
	}

	if value, ok := secret.Data["value"]; ok {
		switch v := value.(type) {
		case string:
			return v, nil
		default:
			return "", errors.New("secret's value is not string")
		}
	} else {
		return "", errors.New("secret's value not found")
	}
}

func NewClient(c *conf.SecretsVault) (*Client, error) {
	url, err := url.Parse(c.Address)
	if err != nil {
		return nil, err
	}
	if url.Scheme != "https" {
		log.Printf("Address uses HTTP scheme which is insecure. It's recommented to use HTTPS instead.")
	}
	cli, err := vault.NewClient(&vault.Config{
		Address: c.Address,
		Timeout: c.Timeout,
	})
	if err != nil {
		return nil, err
	}
	cli.SetToken(c.Token)
	wrapper := &Client{c: cli}
	return wrapper, nil
}

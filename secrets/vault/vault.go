package vault

import (
	"errors"
	"net/http"
	"net/url"

	vault "github.com/hashicorp/vault/api"
	"github.com/mlowicki/rhythm/conf"
	tlsutils "github.com/mlowicki/rhythm/tls"
	log "github.com/sirupsen/logrus"
)

type Client struct {
	c    *vault.Client
	root string
}

func (c *Client) Read(path string) (string, error) {
	secret, err := c.c.Logical().Read(c.root + path)
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
		log.Warnf("Address uses HTTP scheme which is insecure. It's recommented to use HTTPS instead.")
	}
	vc := vault.DefaultConfig()
	vc.Address = c.Address
	vc.Timeout = c.Timeout
	if c.RootCA != "" {
		pool, err := tlsutils.BuildCertPool(c.RootCA)
		if err != nil {
			return nil, err
		}
		tlsConf := vc.HttpClient.Transport.(*http.Transport).TLSClientConfig
		tlsConf.RootCAs = pool
	}
	cli, err := vault.NewClient(vc)
	if err != nil {
		return nil, err
	}
	cli.SetToken(c.Token)
	wrapper := &Client{c: cli, root: c.Root}
	return wrapper, nil
}

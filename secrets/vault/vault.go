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

// Client implements Vault client reading secrets with specified prefix.
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

// New creates fresh instance of Vault client.
func New(c *conf.SecretsVault) (*Client, error) {
	url, err := url.Parse(c.Addr)
	if err != nil {
		return nil, err
	}
	if url.Scheme != "https" {
		log.Warnf("Address uses HTTP scheme which is insecure. It's recommended to use HTTPS instead.")
	}
	vc := vault.DefaultConfig()
	vc.Address = c.Addr
	vc.Timeout = c.Timeout
	if c.CACert != "" {
		pool, err := tlsutils.BuildCertPool(c.CACert)
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

package gitlab

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/mlowicki/rhythm/api/auth"
	"github.com/mlowicki/rhythm/conf"
	tlsutils "github.com/mlowicki/rhythm/tls"
	log "github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

type GitLabAuthorizer struct {
	addr       string
	httpClient *http.Client
}

func New(c *conf.APIAuthGitLab) (*GitLabAuthorizer, error) {
	var httpClient *http.Client
	if c.CACert != "" {
		pool, err := tlsutils.BuildCertPool(c.CACert)
		if err != nil {
			return nil, err
		}
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool},
			},
		}
	}
	if c.Addr == "" {
		return nil, errors.New("GitLab address not set")
	}
	auth := GitLabAuthorizer{
		addr:       c.Addr,
		httpClient: httpClient,
	}
	return &auth, nil
}

func newClient(addr string, token string, httpClient *http.Client) (*gitlab.Client, error) {
	client := gitlab.NewClient(httpClient, token)
	url, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("Error parsing GitLab address: %s\n", err)
	}
	if url.Scheme != "https" {
		log.Warnf("GitLab address uses HTTP scheme which is insecure. It's recommented to use HTTPS instead.")
	}
	err = client.SetBaseURL(addr)
	if err != nil {
		return nil, fmt.Errorf("Error setting GitLab address: %s\n", err)
	}
	return client, nil
}

func (g *GitLabAuthorizer) GetProjectAccessLevel(r *http.Request, group string, project string) (auth.AccessLevel, error) {
	client, err := newClient(g.addr, r.Header.Get("X-Token"), g.httpClient)
	if err != nil {
		return auth.NoAccess, err
	}
	path := fmt.Sprintf("%s/%s", group, project)
	p, _, err := client.Projects.GetProject(path)
	if err != nil {
		switch e := err.(type) {
		case *gitlab.ErrorResponse:
			if e.Response.StatusCode == http.StatusUnauthorized {
				return auth.NoAccess, nil
			}
			return auth.NoAccess, err
		default:
			return auth.NoAccess, err
		}
	}
	perms := p.Permissions
	var lvl gitlab.AccessLevelValue
	if perms.ProjectAccess == nil {
		if perms.GroupAccess == nil {
			return auth.NoAccess, nil
		} else {
			lvl = perms.GroupAccess.AccessLevel
		}
	} else {
		if perms.GroupAccess == nil {
			lvl = perms.ProjectAccess.AccessLevel
		} else {
			if perms.ProjectAccess.AccessLevel >= perms.GroupAccess.AccessLevel {
				lvl = perms.ProjectAccess.AccessLevel
			} else {
				lvl = perms.GroupAccess.AccessLevel
			}
		}
	}
	if lvl >= gitlab.DeveloperPermissions {
		return auth.ReadWrite, nil
	} else if lvl == gitlab.ReporterPermissions {
		return auth.ReadOnly, nil
	} else {
		return auth.NoAccess, nil
	}
}

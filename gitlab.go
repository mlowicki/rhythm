package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/xanzy/go-gitlab"
)

func newGitLabClient(conf *ConfigGitLab, token string) (*gitlab.Client, error) {
	client := gitlab.NewClient(nil, token)
	url, err := url.Parse(conf.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("Error parsing GitLab base URL: %s\n", err)
	}
	if url.Scheme != "https" {
		return nil, errors.New("GitLab base URL must use HTTPS scheme")
	}
	err = client.SetBaseURL(conf.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("Error setting GitLab base URL: %s\n", err)
	}
	return client, nil
}

type GitLabAuthorizer struct {
	Config *ConfigGitLab
}

func (g *GitLabAuthorizer) GetProjectAccessLevel(r *http.Request, group string, project string) (AccessLevel, error) {
	client, err := newGitLabClient(g.Config, r.Header.Get("X-Token"))
	if err != nil {
		return NoAccess, err
	}
	path := fmt.Sprintf("%s/%s", group, project)
	p, _, err := client.Projects.GetProject(path)
	if err != nil {
		switch e := err.(type) {
		case *gitlab.ErrorResponse:
			if e.Response.StatusCode == http.StatusUnauthorized {
				return NoAccess, nil
			}
			return NoAccess, err
		default:
			return NoAccess, err
		}
	}
	perms := p.Permissions
	var lvl gitlab.AccessLevelValue
	if perms.ProjectAccess == nil {
		if perms.GroupAccess == nil {
			return NoAccess, nil
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
		return ReadWrite, nil
	} else if lvl == gitlab.ReporterPermissions {
		return ReadOnly, nil
	} else {
		return NoAccess, nil
	}
}

package main

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/xanzy/go-gitlab"
)

type AccessLevel int

const (
	NoAccess AccessLevel = iota
	ReadOnly
	ReadWrite
)

func newGitLabClient(conf *Config, token string) (*gitlab.Client, error) {
	client := gitlab.NewClient(nil, token)
	url, err := url.Parse(conf.GitLab.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("Error parsing GitLab base URL: %s\n", err)
	}
	if url.Scheme != "https" {
		return nil, errors.New("GitLab base URL must use HTTPS scheme")
	}
	err = client.SetBaseURL(conf.GitLab.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("Error setting GitLab base URL: %s\n", err)
	}
	return client, nil
}

func getProjectAccessLevel(p *gitlab.Project) AccessLevel {
	perms := p.Permissions
	var lvl gitlab.AccessLevelValue

	if perms.ProjectAccess == nil {
		if perms.GroupAccess == nil {
			return NoAccess
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
		return ReadWrite
	} else if lvl == gitlab.ReporterPermissions {
		return ReadOnly
	} else {
		return NoAccess
	}
}

package main

import (
	"log"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
	"github.com/mlowicki/rhythm/conf"
)

func getMesosHTTPClient(c *conf.Mesos) *httpcli.Client {
	var authConf httpcli.ConfigOpt

	if c.Auth.Type == conf.MesosAuthModeBasic {
		authConf = httpcli.BasicAuth(c.Auth.Basic.Username, c.Auth.Basic.Password)
	} else if c.Auth.Type != conf.MesosAuthModeNone {
		log.Fatalf("Unknown authentication mode: %s\n", c.Auth.Type)
	}

	return httpcli.New(
		httpcli.Endpoint(c.BaseURL+"/api/v1/scheduler"),
		httpcli.Do(httpcli.With(
			authConf,
			httpcli.Timeout(time.Second*10),
		)))
}

func getFrameworkInfo(conf *conf.Mesos) *mesos.FrameworkInfo {
	// https://github.com/apache/mesos/blob/master/include/mesos/mesos.proto
	// TODO Option to set `roles` (or `role`)
	// TODO Option to set `capabilities`
	// TODO Option to set `labels`
	return &mesos.FrameworkInfo{
		User:            conf.User,
		Name:            name,
		Checkpoint:      &conf.Checkpoint,
		Capabilities:    []mesos.FrameworkInfo_Capability{},
		Labels:          &mesos.Labels{},
		FailoverTimeout: func() *float64 { ft := conf.FailoverTimeout.Seconds(); return &ft }(),
		WebUiURL:        &conf.WebUiURL,
		Hostname:        &conf.Hostname,
		Principal:       &conf.Principal,
	}
}

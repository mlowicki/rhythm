package main

import (
	"log"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
)

func getMesosHTTPClient(conf *ConfigMesos) *httpcli.Client {
	var authConf httpcli.ConfigOpt

	if conf.Auth.Type == AuthModeBasic {
		authConf = httpcli.BasicAuth(conf.Auth.Basic.Username, conf.Auth.Basic.Password)
	} else if conf.Auth.Type != AuthModeNone {
		log.Fatalf("Unknown authentication mode: %s\n", conf.Auth.Type)
	}

	return httpcli.New(
		httpcli.Endpoint(conf.BaseURL+"/api/v1/scheduler"),
		httpcli.Do(httpcli.With(
			authConf,
			httpcli.Timeout(time.Second*10),
		)))
}

func getFrameworkInfo(conf *ConfigMesos) *mesos.FrameworkInfo {
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
		FailoverTimeout: &conf.FailoverTimeout,
		WebUiURL:        &conf.WebUiURL,
		Hostname:        &conf.Hostname,
		Principal:       &conf.Principal,
	}
}

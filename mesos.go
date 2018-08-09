package main

import (
	"log"
	"time"

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

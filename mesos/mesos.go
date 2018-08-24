package mesos

import (
	"log"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
	"github.com/mlowicki/rhythm/conf"
)

const frameworkName = "rhythm"

func NewHTTPClient(c *conf.Mesos) *httpcli.Client {
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

func NewFrameworkInfo(conf *conf.Mesos, idStore store.Singleton) *mesos.FrameworkInfo {
	// https://github.com/apache/mesos/blob/master/include/mesos/mesos.proto
	// TODO Option to set `roles` (or `role`)
	// TODO Option to set `capabilities`
	// TODO Option to set `labels`
	frameworkInfo := &mesos.FrameworkInfo{
		User:            conf.User,
		Name:            frameworkName,
		Checkpoint:      &conf.Checkpoint,
		Capabilities:    []mesos.FrameworkInfo_Capability{},
		Labels:          &mesos.Labels{},
		FailoverTimeout: func() *float64 { ft := conf.FailoverTimeout.Seconds(); return &ft }(),
		WebUiURL:        &conf.WebUiURL,
		Hostname:        &conf.Hostname,
		Principal:       &conf.Principal,
	}
	id, _ := idStore.Get()
	frameworkInfo.ID = &mesos.FrameworkID{Value: *proto.String(id)}
	return frameworkInfo
}

type storage interface {
	SetFrameworkID(id string) error
	GetFrameworkID() (string, error)
}

func NewFrameworkIDStore(s storage) (store.Singleton, error) {
	fidStore := store.NewInMemorySingleton()
	fid, err := s.GetFrameworkID()
	if err != nil {
		return nil, err
	}
	if fid != "" {
		log.Printf("Framework ID: %s\n", fid)
		fidStore.Set(fid)
	}
	return store.DecorateSingleton(
		fidStore,
		store.DoSet().AndThen(func(_ store.Setter, v string, _ error) error {
			log.Printf("Framework ID: %s\n", v)
			err := s.SetFrameworkID(v)
			return err
		})), nil
}

package mesos

import (
	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	log "github.com/sirupsen/logrus"
)

const frameworkName = "rhythm"

type secrets interface {
	Read(string) (string, error)
}

type storage interface {
	GetJobs() ([]*model.Job, error)
	GetJob(group string, project string, id string) (*model.Job, error)
	SetFrameworkID(id string) error
	GetFrameworkID() (string, error)
	SaveJob(j *model.Job) error
	AddTask(group, project, id string, task *model.Task) error
	GetJobRuntime(group, project, id string) (*model.JobRuntime, error)
	SaveJobRuntime(group, project, id string, state *model.JobRuntime) error
	GetJobConf(group, project, id string) (*model.JobConf, error)
	SaveJobConf(state *model.JobConf) error
}

func newFrameworkInfo(conf *conf.Mesos, idStore store.Singleton) *mesos.FrameworkInfo {
	labels := make([]mesos.Label, len(conf.Labels))
	for k, v := range conf.Labels {
		func(v string) {
			labels = append(labels, mesos.Label{Key: k, Value: &v})
		}(v)
	}
	frameworkInfo := &mesos.FrameworkInfo{
		User:       conf.User,
		Name:       frameworkName,
		Checkpoint: &conf.Checkpoint,
		Capabilities: []mesos.FrameworkInfo_Capability{
			{Type: mesos.FrameworkInfo_Capability_MULTI_ROLE},
		},
		Labels:          &mesos.Labels{labels},
		FailoverTimeout: func() *float64 { ft := conf.FailoverTimeout.Seconds(); return &ft }(),
		WebUiURL:        &conf.WebUiURL,
		Hostname:        &conf.Hostname,
		Principal:       &conf.Principal,
		Roles:           conf.Roles,
	}
	id, _ := idStore.Get()
	frameworkInfo.ID = &mesos.FrameworkID{Value: *proto.String(id)}
	return frameworkInfo
}

func newFrameworkIDStore(s storage) (store.Singleton, error) {
	fidStore := store.NewInMemorySingleton()
	fid, err := s.GetFrameworkID()
	if err != nil {
		return nil, err
	}
	if fid != "" {
		log.Printf("Framework ID: %s", fid)
		fidStore.Set(fid)
	}
	return store.DecorateSingleton(
		fidStore,
		store.DoSet().AndThen(func(_ store.Setter, v string, _ error) error {
			log.Printf("Framework ID: %s", v)
			err := s.SetFrameworkID(v)
			return err
		})), nil
}

package mesos

import (
	"fmt"

	"github.com/gofrs/uuid"
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
	GetRunnableJobs() ([]*model.Job, error)
	SaveJob(j *model.Job) error
}

func newFrameworkInfo(conf *conf.Mesos, idStore store.Singleton) *mesos.FrameworkInfo {
	// https://github.com/apache/mesos/blob/master/include/mesos/mesos.proto
	// TODO Option to set `roles` (or `role`)
	// TODO Option to set `capabilities`
	labels := make([]mesos.Label, len(conf.Labels))
	for k, v := range conf.Labels {
		func(v string) {
			labels = append(labels, mesos.Label{Key: k, Value: &v})
		}(v)
	}
	frameworkInfo := &mesos.FrameworkInfo{
		User:            conf.User,
		Name:            frameworkName,
		Checkpoint:      &conf.Checkpoint,
		Capabilities:    []mesos.FrameworkInfo_Capability{},
		Labels:          &mesos.Labels{labels},
		FailoverTimeout: func() *float64 { ft := conf.FailoverTimeout.Seconds(); return &ft }(),
		WebUiURL:        &conf.WebUiURL,
		Hostname:        &conf.Hostname,
		Principal:       &conf.Principal,
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

func newTaskInfo(j *model.Job, sec secrets) (error, *mesos.TaskInfo) {
	u4, err := uuid.NewV4()
	if err != nil {
		return err, nil
	}
	id := fmt.Sprintf("%s:%s:%s:%s", j.Group, j.Project, j.ID, u4)
	env := mesos.Environment{
		Variables: []mesos.Environment_Variable{
			{Name: "TASK_ID", Value: &id},
		},
	}
	for k, v := range j.Env {
		env.Variables = append(env.Variables, mesos.Environment_Variable{Name: k, Value: func(v string) *string { return &v }(v)})
	}
	for k, v := range j.Secrets {
		path := fmt.Sprintf("%s/%s/%s/%s", j.Group, j.Project, j.ID, v)
		secret, err := sec.Read(path)
		if err != nil {
			return err, nil
		}
		env.Variables = append(env.Variables, mesos.Environment_Variable{Name: k, Value: &secret})
	}
	var containerInfo mesos.ContainerInfo
	switch j.Container.Kind {
	case model.Docker:
		containerInfo = mesos.ContainerInfo{
			Type: mesos.ContainerInfo_DOCKER.Enum(),
			Docker: &mesos.ContainerInfo_DockerInfo{
				Image: j.Container.Docker.Image,
			},
		}
	case model.Mesos:
		containerInfo = mesos.ContainerInfo{
			Type: mesos.ContainerInfo_MESOS.Enum(),
			Docker: &mesos.ContainerInfo_DockerInfo{
				Image: j.Container.Mesos.Image,
			},
		}
	default:
		log.Fatalf("Unknown container type: %d", j.Container.Kind)
	}
	labels := make([]mesos.Label, len(j.Labels))
	for k, v := range j.Labels {
		func(v string) {
			labels = append(labels, mesos.Label{Key: k, Value: &v})
		}(v)
	}
	task := mesos.TaskInfo{
		TaskID: mesos.TaskID{Value: id},
		Command: &mesos.CommandInfo{
			Value:       proto.String(j.Cmd),
			Environment: &env,
			User:        proto.String(j.User),
			Shell:       proto.Bool(j.Shell),
			Arguments:   j.Arguments,
		},
		Container: &containerInfo,
		Labels:    &mesos.Labels{labels},
	}
	task.Name = "Task " + task.TaskID.Value
	return nil, &task
}

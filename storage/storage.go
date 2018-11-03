package storage

import (
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	"github.com/mlowicki/rhythm/storage/zk"
	log "github.com/sirupsen/logrus"
)

type storage interface {
	DeleteJob(group string, project string, id string) error
	GetFrameworkID() (string, error)
	GetGroupJobs(group string) ([]*model.Job, error)
	GetJob(group string, project string, id string) (*model.Job, error)
	GetJobs() ([]*model.Job, error)
	GetProjectJobs(group string, project string) ([]*model.Job, error)
	SaveJob(j *model.Job) error
	SetFrameworkID(id string) error
	AddTask(group, project, id string, task *model.Task) error
	GetTasks(group string, project string, id string) ([]*model.Task, error)
	GetJobRuntime(group, project, id string) (*model.JobRuntime, error)
	SaveJobRuntime(group, project, id string, state *model.JobRuntime) error
	GetJobConf(group, project, id string) (*model.JobConf, error)
	SaveJobConf(state *model.JobConf) error
}

func New(c *conf.Storage) storage {
	if c.Backend == conf.StorageBackendZK {
		s, err := zk.NewStorage(&c.ZooKeeper)
		if err != nil {
			log.Fatal(err)
		}
		return s
	} else {
		log.Fatalf("Unknown backend: %s", c.Backend)
		return nil
	}
}

package storage

import (
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	"github.com/mlowicki/rhythm/storage/zk"
	log "github.com/sirupsen/logrus"
)

type storage interface {
	DeleteJob(group, project, id string) error
	GetFrameworkID() (string, error)
	GetGroupJobs(group string) ([]*model.Job, error)
	GetJob(group, project, id string) (*model.Job, error)
	GetJobs() ([]*model.Job, error)
	GetProjectJobs(group, project string) ([]*model.Job, error)
	SaveJob(j *model.Job) error
	SetFrameworkID(id string) error
	AddTask(group, project, id string, task *model.Task) error
	GetTasks(group, project, id string) ([]*model.Task, error)
	GetJobRuntime(group, project, id string) (*model.JobRuntime, error)
	SaveJobRuntime(group, project, id string, state *model.JobRuntime) error
	GetJobConf(group, project, id string) (*model.JobConf, error)
	SaveJobConf(state *model.JobConf) error
	QueueJob(group, project, id string) error
	DequeueJob(group, project, id string) error
	GetQueuedJobsIDs() ([]model.JobID, error)
}

func New(c *conf.Storage) storage {
	if c.Backend == conf.StorageBackendZK {
		s, err := zk.New(&c.ZooKeeper)
		if err != nil {
			log.Fatal(err)
		}
		return s
	}
	log.Fatalf("Unknown backend: %s", c.Backend)
	return nil
}

package storage

import (
	"log"

	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	"github.com/mlowicki/rhythm/storage/zk"
)

type storage interface {
	DeleteJob(group string, project string, id string) error
	GetFrameworkID() (string, error)
	GetGroupJobs(group string) ([]*model.Job, error)
	GetJob(group string, project string, id string) (*model.Job, error)
	GetJobs() ([]*model.Job, error)
	GetProjectJobs(group string, project string) ([]*model.Job, error)
	GetRunnableJobs() ([]*model.Job, error)
	SaveJob(j *model.Job) error
	SetFrameworkID(id string) error
}

func New(c *conf.Storage) storage {
	if c.Type == conf.StorageTypeZooKeeper {
		s, err := zk.NewStorage(&c.ZooKeeper)
		if err != nil {
			log.Fatalf("Error initializing ZooKeeper storage: %s\n", err)
		}
		return s
	} else {
		log.Fatalf("Unknown storage type: %s\n", c.Type)
		return nil
	}
}

package main

import "github.com/mlowicki/rhythm/model"

type Storage interface {
	GetJobs() ([]*model.Job, error)
	GetJob(group string, project string, id string) (*model.Job, error)
	SetFrameworkID(id string) error
	GetFrameworkID() (string, error)
	GetRunnableJobs() ([]*model.Job, error)
	SaveJob(j *model.Job) error
}

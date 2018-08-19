package main

type Storage interface {
	SetFrameworkID(id string) error
	GetFrameworkID() (string, error)
	GetJobs() ([]*Job, error)
	GetGroupJobs(group string) ([]*Job, error)
	GetProjectJobs(group string, project string) ([]*Job, error)
	GetJob(group string, project string, id string) (*Job, error)
	GetRunnableJobs() ([]*Job, error)
	SaveJob(j *Job) error
	DeleteJob(group string, project string, id string) error
}

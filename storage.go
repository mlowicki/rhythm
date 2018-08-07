package main

type Storage interface {
	SetFrameworkID(id string) error
	GetFrameworkID() (string, error)
	GetJob(group string, project string, id string) (*Job, error)
	GetRunnableJobs() ([]*Job, error)
	SaveJob(j *Job) error
}

package model

import (
	"fmt"
	"time"
)

type State int

const (
	IDLE State = iota
	STAGING
	STARTING
	RUNNING
	FAILED
)

type JobDocker struct {
	Image string
}

type JobContainer struct {
	Kind   ContainerKind
	Docker JobDocker
}

type ContainerKind int

const (
	Docker ContainerKind = iota
	Mesos
)

type JobSchedule struct {
	Kind ScheduleKind
	Cron string
}

type ScheduleKind int

const (
	Cron ScheduleKind = iota
)

// TODO Support for force pull time in Docker
// TODO Support for custom args like Docker ENTRYPOINT
type Job struct {
	Group       string
	Project     string
	ID          string
	Schedule    JobSchedule
	CreatedAt   time.Time
	LastStartAt time.Time
	Env         map[string]string
	Secrets     map[string]string
	Container   JobContainer
	State       State
	LastFail    LastFail
	CPUs        float64
	Mem         float64
	Cmd         string
	User        string
}

type LastFail struct {
	Message string
	Reason  string
	Source  string
	When    time.Time
}

func (j *Job) String() string {
	return fmt.Sprintf("%s:%s:%s", j.Group, j.Project, j.ID)
}

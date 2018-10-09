package model

import (
	"fmt"
	"time"
)

type State string

const (
	IDLE     State = "Idle"
	STAGING        = "Staging"
	STARTING       = "Starting"
	RUNNING        = "Running"
	FAILED         = "Failed"
)

type JobDocker struct {
	Image          string
	ForcePullImage bool
}

type JobMesos struct {
	Image string
}

type JobContainer struct {
	Type   ContainerType
	Docker *JobDocker `json:",omitempty"`
	Mesos  *JobMesos  `json:",omitempty"`
}

type ContainerType string

const (
	Docker ContainerType = "Docker"
	Mesos                = "Mesos"
)

type JobSchedule struct {
	Type ScheduleType
	Cron string `json:",omitempty"`
}

type ScheduleType string

const (
	Cron ScheduleType = "Cron"
)

type Job struct {
	Group       string
	Project     string
	ID          string
	Schedule    JobSchedule
	CreatedAt   time.Time
	LastStartAt time.Time
	TaskID      string
	AgentID     string
	Env         map[string]string
	Secrets     map[string]string
	Container   JobContainer
	State       State
	LastFail    LastFail
	CPUs        float64
	Mem         float64
	Cmd         string
	User        string
	Shell       bool
	Arguments   []string
	Labels      map[string]string
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

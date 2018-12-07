package model

import (
	"fmt"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

var CronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type State string

const (
	IDLE     State = "Idle"
	STAGING        = "Staging"
	STARTING       = "Starting"
	RUNNING        = "Running"
	FAILED         = "Failed"
)

func (s State) String() string {
	return string(s)
}

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

type JobConf struct {
	JobID
	Schedule   JobSchedule
	Env        map[string]string
	Secrets    map[string]string
	Container  JobContainer
	CPUs       float64
	Mem        float64
	Disk       float64
	Cmd        string
	User       string
	Shell      bool
	Arguments  []string
	Labels     map[string]string
	MaxRetries int
}

func (j *JobConf) String() string {
	return j.FQID()
}

func (j *JobConf) Resources() mesos.Resources {
	res := mesos.Resources{}
	res.Add(
		resources.NewCPUs(j.CPUs).Resource,
		resources.NewMemory(j.Mem).Resource,
		resources.NewDisk(j.Disk).Resource,
	)
	return res
}

type JobRuntime struct {
	State          State
	LastStart      time.Time
	CurrentTaskID  string
	CurrentAgentID string
	Retries        int
}

type Job struct {
	JobConf
	JobRuntime
}

func (job *Job) NextRun() time.Time {
	if job.IsRetryable() {
		return job.LastStart
	}
	if job.Schedule.Type != Cron {
		log.Panic("Only Cron schedule is supported")
	}
	sched, err := CronParser.Parse(job.Schedule.Cron)
	if err != nil {
		log.Panic(err)
	}
	return sched.Next(job.LastStart)
}

func (job *Job) IsRetryable() bool {
	return job.State == FAILED && job.Retries < job.MaxRetries
}

func (job *Job) IsRunnable() bool {
	return (job.State == IDLE || job.State == FAILED) && job.NextRun().Before(time.Now())
}

type Task struct {
	Start       time.Time
	End         time.Time
	TaskID      string
	ExecutorID  string
	AgentID     string
	FrameworkID string
	ExecutorURL string
	// Set for failed task
	Message string
	Reason  string
	Source  string
}

type JobID struct {
	Group   string
	Project string
	ID      string
}

// Fully qualified identifier unique across jobs from all groups and projects.
func (jid *JobID) FQID() string {
	return fmt.Sprintf("%s:%s:%s", jid.Group, jid.Project, jid.ID)
}

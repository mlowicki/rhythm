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
	Group     string
	Project   string
	ID        string
	Schedule  JobSchedule
	Env       map[string]string
	Secrets   map[string]string
	Container JobContainer
	CPUs      float64
	Mem       float64
	Disk      float64
	Cmd       string
	User      string
	Shell     bool
	Arguments []string
	Labels    map[string]string
}

func (j *JobConf) String() string {
	return j.FQID()
}

// Fully qualified identifier unique across jobs from all groups and projects.
func (j *JobConf) FQID() string {
	return fmt.Sprintf("%s:%s:%s", j.Group, j.Project, j.ID)
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
}

type Job struct {
	JobConf
	JobRuntime
}

func (j *Job) NextRun() time.Time {
	if j.Schedule.Type != Cron {
		log.Panic("Only Cron schedule is supported")
	}
	sched, err := CronParser.Parse(j.Schedule.Cron)
	if err != nil {
		log.Panic(err)
	}
	var t time.Time
	if j.LastStart.IsZero() {
		now := time.Now()
		t = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	} else {
		t = j.LastStart
	}
	return sched.Next(t)
}

func (j *Job) IsRunnable() bool {
	if j.State != IDLE && j.State != FAILED {
		return false
	}
	return j.NextRun().Before(time.Now())
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

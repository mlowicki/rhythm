package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

// CronParser is a package-level parser for cron syntax.
var CronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// State defines job position.
type State string

const (
	// IDLE denotes not running job which either hasn't been scheduled yet or its last run was successful.
	IDLE State = "Idle"
	// STAGING denotes job which has been scheduled to run in response to offer.
	STAGING = "Staging"
	// STARTING denotes job which has been lanunched by executor.
	STARTING = "Starting"
	// RUNNING denotes job which has been started.
	RUNNING = "Running"
	// FAILED denotes job whose last run failed.
	FAILED = "Failed"
)

func (s State) String() string {
	return string(s)
}

// JobDocker defines fields for job running inside Docker container.
type JobDocker struct {
	Image          string
	ForcePullImage bool
}

// JobMesos defines fields for job running inside Mesos container.
// https://mesos.apache.org/documentation/latest/mesos-containerizer/
type JobMesos struct {
	Image string
}

// JobContainer defines containerizer-related fields for job.
type JobContainer struct {
	Type   ContainerType
	Docker *JobDocker `json:",omitempty"`
	Mesos  *JobMesos  `json:",omitempty"`
}

// ContainerType defines containerizer genre.
type ContainerType string

const (
	// Docker denotes Docker containerizer.
	Docker ContainerType = "Docker"
	// Mesos denotes Mesos containerizer.
	// https://mesos.apache.org/documentation/latest/container-image/
	Mesos = "Mesos"
)

// JobsSchedule defines fields related to job's timetable.
type JobSchedule struct {
	Type ScheduleType
	Cron string `json:",omitempty"`
}

// ScheduleType defines timetable genre.
type ScheduleType string

const (
	// Cron denotes cron-like timetable.
	Cron ScheduleType = "Cron"
)

// JobConf defines job's configuration fields.
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

// FQID returns globablly unique identifier (acrsoss all groups and projects).
func (j *JobConf) FQID() string {
	return j.JobID.String()
}

func (j *JobConf) String() string {
	return j.FQID()
}

// Resources returns resources required by job.
func (j *JobConf) Resources() mesos.Resources {
	res := mesos.Resources{}
	res.Add(
		resources.NewCPUs(j.CPUs).Resource,
		resources.NewMemory(j.Mem).Resource,
		resources.NewDisk(j.Disk).Resource,
	)
	return res
}

// JobConf defines job's runtime fields.
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

// NextRun returnes time when job should be launched.
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

// IsRetryable returns true if job's last run failed and job is eligible for retry.
func (job *Job) IsRetryable() bool {
	return job.State == FAILED && job.Retries < job.MaxRetries
}

// IsRunnable returns true if job should be launched.
func (job *Job) IsRunnable() bool {
	return (job.State == IDLE || job.State == FAILED) && job.NextRun().Before(time.Now())
}

// Task is a single run (failed or successful) of job.
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

// JobID defines job identifier.
type JobID struct {
	Group   string
	Project string
	ID      string
}

// Fully qualified identifier unique across jobs from all groups and projects.
func (jid *JobID) String() string {
	return fmt.Sprintf("%s:%s:%s", jid.Group, jid.Project, jid.ID)
}

// ParseJobID parses serialized job ID returned by String JobID.String method.
func ParseJobID(v string) (*JobID, error) {
	chunks := strings.Split(v, ":")
	if len(chunks) != 3 {
		return nil, errors.New("Invalid number of chunks")
	}
	jid := JobID{
		Group:   chunks[0],
		Project: chunks[1],
		ID:      chunks[2],
	}
	return &jid, nil
}

package jobsscheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/mlowicki/rhythm/model"
	log "github.com/sirupsen/logrus"
)

const srcScheduler = "Scheduler"

type secrets interface {
	Read(string) (string, error)
}

type storage interface {
	GetJobs() ([]*model.Job, error)
	AddTask(group, project, id string, task *model.Task) error
	SaveJobRuntime(group, project, id string, state *model.JobRuntime) error
	GetQueuedJobsIDs() ([]model.JobID, error)
	DequeueJob(group, project, id string) error
}

// Scheduler decides which jobs to run in response to received offers.
type Scheduler struct {
	roles       []string
	storage     storage
	secrets     secrets
	frameworkID func() string
	leaderURL   func() string
	// In-memory cache of all jobs.
	jobs    map[string]*model.Job
	jobsMut sync.Mutex
	// Indicates if job has been selected by scheduler for one
	// of received offers but offer hasn't been accepted yet.
	// Used to ensure to run simultaneously maximum one task per job.
	bookedJobs *ttlSet
	// Queued job is scheduled for immediate run.
	queuedJobs    map[string]struct{}
	queuedJobsMut sync.Mutex
}

func (sched *Scheduler) getJob(jid string) (model.Job, bool) {
	sched.jobsMut.Lock()
	job, ok := sched.jobs[jid]
	sched.jobsMut.Unlock()
	return *job, ok
}

func (sched *Scheduler) dequeueJob(job *model.Job) {
	fqid := job.FQID()
	sched.queuedJobsMut.Lock()
	_, isQueued := sched.queuedJobs[fqid]
	sched.queuedJobsMut.Unlock()
	if isQueued {
		err := sched.storage.DequeueJob(job.Group, job.Project, job.ID)
		if err != nil {
			log.Errorf("Error dequeuing job: %s", err)
		}
		sched.queuedJobsMut.Lock()
		delete(sched.queuedJobs, job.FQID())
		sched.queuedJobsMut.Unlock()
	}
}

func (sched *Scheduler) setJob(job model.Job) {
	sched.jobsMut.Lock()
	sched.jobs[job.FQID()] = &job
	sched.jobsMut.Unlock()
}

// New creates fresh instance of jobs scheduler.
func New(ctx context.Context, roles []string, stor storage, secr secrets, frameworkID, leaderURL func() string) *Scheduler {
	sched := Scheduler{
		roles:       roles,
		storage:     stor,
		secrets:     secr,
		frameworkID: frameworkID,
		leaderURL:   leaderURL,
		jobs:        make(map[string]*model.Job),
		bookedJobs:  newTTLSet(time.Minute),
	}
	sync := func() {
		sched.syncJobsCache()
		sched.syncQueuedJobsCache()
	}
	sync()
	go func() {
		interval := time.Second * 30
		timer := time.After(interval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer:
				sync()
				timer = time.After(interval)
			}
		}
	}()
	return &sched
}

func (sched *Scheduler) syncQueuedJobsCache() {
	log.Debugf("Queued jobs cache syncing...")
	var jids []model.JobID
	for {
		var err error
		jids, err = sched.storage.GetQueuedJobsIDs()
		if err == nil {
			break
		}
		log.Error(err)
		<-time.After(time.Second)
	}
	queuedJobs := make(map[string]struct{}, len(jids))
	for _, jid := range jids {
		queuedJobs[jid.String()] = struct{}{}
	}
	sched.queuedJobsMut.Lock()
	sched.queuedJobs = queuedJobs
	sched.queuedJobsMut.Unlock()
	log.Debugf("Queued jobs cache synced")
}

func (sched *Scheduler) syncJobsCache() {
	log.Debugf("Jobs cache syncing...")
	var newJobs []*model.Job
	for {
		var err error
		newJobs, err = sched.storage.GetJobs()
		if err == nil {
			break
		}
		log.Error(err)
		<-time.After(time.Second)
	}
	sched.jobsMut.Lock()
	ids := make(map[string]struct{}, len(newJobs))
	for _, job := range newJobs {
		id := job.FQID()
		ids[id] = struct{}{}
		oldJob, ok := sched.jobs[id]
		if ok {
			// Modify only conf since running instance has most up-to-date
			// runtime state as saving updates in storage can fail.
			oldJob.JobConf = job.JobConf
		} else {
			sched.jobs[id] = job
		}
	}
	// Evict from cache jobs not present in storage.
	for _, job := range sched.jobs {
		id := job.FQID()
		_, ok := ids[id]
		if !ok {
			delete(sched.jobs, id)
		}
	}
	sched.jobsMut.Unlock()
	log.Debugf("Jobs cache synced")
}

// HandleTaskStateUpdate processes task state change.
func (sched *Scheduler) HandleTaskStateUpdate(status *mesos.TaskStatus) {
	tid := status.TaskID.Value
	jid, err := parseTaskID(tid)
	if err != nil {
		log.WithFields(log.Fields{
			"taskID": tid,
		}).Errorf("Error getting job ID from task ID: %s", err)
		return
	}
	state := status.GetState()
	log.Debugf("Task state update: %s (%s)", tid, state)
	job, ok := sched.getJob(jid.String())
	if !ok {
		log.Printf("Update for unknown job: %s", jid)
	}
	switch state {
	case mesos.TASK_STAGING:
		job.State = model.STAGING
	case mesos.TASK_STARTING:
		job.State = model.STARTING
	case mesos.TASK_RUNNING:
		job.State = model.RUNNING
	case mesos.TASK_FINISHED:
		log.Debugf("Task finished successfully: %s", status.TaskID.Value)
		sched.addTaskHistory(status, job.LastStart, jid)
		job.State = model.IDLE
		job.CurrentTaskID = ""
		job.CurrentAgentID = ""
	case mesos.TASK_LOST:
		/*
		 * 1. Reconciliation run gets running task A
		 * 2. Task A finishes successfully
		 * 3. Reconciliation for task A sent
		 * 4. TASK_LOST is received which would mark job A as failed
		 *
		 * It's still a small window when it's possible = if handler for TASK_LOST
		 * read from storage before handler for e.g. TASK_FINISHED persisted update.
		 */
		if status.GetReason() == mesos.REASON_RECONCILIATION {
			if job.State == model.IDLE || job.State == model.FAILED {
				return
			}
		}
		fallthrough
	case mesos.TASK_FAILED:
		fallthrough
	case mesos.TASK_KILLED:
		fallthrough
	case mesos.TASK_ERROR:
		msg := status.GetMessage()
		reason := status.GetReason().String()
		src := status.GetSource().String()
		state := status.GetState()
		log.Errorf("Task failed: %s (%s; %s; %s; %s)", tid, state, msg, reason, src)
		sched.addTaskHistory(status, job.LastStart, jid)
		job.State = model.FAILED
		job.CurrentTaskID = ""
		job.CurrentAgentID = ""
	default:
		log.Panicf("Unknown state: %s", state)
	}
	sched.setJob(job)
	err = sched.storage.SaveJobRuntime(jid.Group, jid.Project, jid.ID, &job.JobRuntime)
	if err != nil {
		log.Errorf("Error saving job while handling update: %s", err)
	}
}

func (sched *Scheduler) findTaskResources(taskResources, offerResources mesos.Resources) mesos.Resources {
	var found mesos.Resources
	role := sched.roles[0]
	if role == "*" {
		found = resources.Find(taskResources, offerResources...)
	} else {
		reservation := mesos.Resource_ReservationInfo{
			Type: mesos.Resource_ReservationInfo_STATIC.Enum(),
			Role: &role,
		}
		found = resources.Find(taskResources.PushReservation(reservation), offerResources...)
	}
	return found
}

// FindTasksForOffer returns tasks to run for passed offer.
func (sched *Scheduler) FindTasksForOffer(ctx context.Context, offer *mesos.Offer) []mesos.TaskInfo {
	rs := mesos.Resources(offer.Resources)
	log.Debugf("Finding tasks for offer: %s", rs)
	jobs, jobsRs := sched.findJobsForResources(rs)
	log.Debugf("Found %d tasks for offer", len(jobs))
	tasks := sched.buildTasksForOffer(jobs, jobsRs, offer)
	return tasks
}

/**
 * Find jobs to run for specified resources.
 *
 * Returns two slices:
 * - jobs to run
 * - resources to use for respective job from 1st slice
 */
func (sched *Scheduler) findJobsForResources(res mesos.Resources) ([]model.Job, []mesos.Resources) {
	var tasksRes []mesos.Resources
	var jobs []model.Job
	res = res.Unallocate()
	resUnreserved := res.ToUnreserved()
	sched.jobsMut.Lock()
	sched.queuedJobsMut.Lock()
	for _, job := range sched.jobs {
		if sched.bookedJobs.Exists(job.FQID()) {
			continue
		}
		if _, isQueued := sched.queuedJobs[job.FQID()]; !job.IsRunnable() && !isQueued {
			continue
		}
		jobRes := job.Resources()
		if !resources.ContainsAll(resUnreserved, jobRes) {
			continue
		}
		if job.IsRetryable() {
			job.Retries += 1
		} else {
			job.Retries = 0
		}
		taskRes := sched.findTaskResources(jobRes, res)
		if len(taskRes) == 0 {
			log.Fatal("Resources not found")
		}
		log.Debugf("Found resources for job: %s", job)
		jobs = append(jobs, *job)
		tasksRes = append(tasksRes, taskRes)
		res.Subtract(taskRes...)
		resUnreserved = res.ToUnreserved()
		sched.bookedJobs.Set(job.FQID())
	}
	sched.queuedJobsMut.Unlock()
	sched.jobsMut.Unlock()
	return jobs, tasksRes
}

func (sched *Scheduler) buildTasksForOffer(jobs []model.Job, ress []mesos.Resources, offer *mesos.Offer) []mesos.TaskInfo {
	var tasks []mesos.TaskInfo
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(i int, job *model.Job) {
			defer wg.Done()
			job.LastStart = time.Now()
			task, err := sched.newTaskInfo(job)
			if err != nil {
				log.Errorf("Error creating TaskInfo: %s", err)
				job.State = model.FAILED
				go func() {
					now := time.Now()
					task := model.Task{
						Start:   now,
						End:     now,
						Message: err.Error(),
						Reason:  "Error creating TaskInfo",
						Source:  srcScheduler,
					}
					err := sched.storage.AddTask(job.Group, job.Project, job.ID, &task)
					if err != nil {
						log.Errorf("Error saving task: %s", err)
					}
				}()
			} else {
				job.State = model.STAGING
				job.CurrentTaskID = task.TaskID.GetValue()
				job.CurrentAgentID = offer.AgentID.GetValue()
				task.AgentID = offer.AgentID
				task.Resources = ress[i]
				tasks = append(tasks, *task)
			}
			err = sched.storage.SaveJobRuntime(job.Group, job.Project, job.ID, &job.JobRuntime)
			if err != nil {
				log.Errorf("Error updating job runtime info: %s", err)
			}
			sched.setJob(*job)
			sched.dequeueJob(job)
			sched.bookedJobs.Del(job.FQID())
		}(i, &jobs[i])
	}
	wg.Wait()
	return tasks
}

func strPtr(v string) *string { return &v }

func (sched *Scheduler) newTaskInfo(job *model.Job) (*mesos.TaskInfo, error) {
	tid, err := newTaskID(&job.JobID)
	if err != nil {
		return nil, fmt.Errorf("Getting task ID failed: %s", err)
	}
	env := mesos.Environment{
		Variables: []mesos.Environment_Variable{
			{Name: "RHYTHM_TASK_ID", Value: &tid},
			{Name: "RHYTHM_MEM", Value: strPtr(fmt.Sprintf("%g", job.Mem))},
			{Name: "RHYTHM_DISK", Value: strPtr(fmt.Sprintf("%g", job.Disk))},
			{Name: "RHYTHM_CPU", Value: strPtr(fmt.Sprintf("%g", job.CPUs))},
		},
	}
	for k, v := range job.Env {
		envvar := mesos.Environment_Variable{Name: k, Value: strPtr(v)}
		env.Variables = append(env.Variables, envvar)
	}
	for k, v := range job.Secrets {
		path := fmt.Sprintf("%s/%s/%s", job.Group, job.Project, v)
		secret, err := sched.secrets.Read(path)
		if err != nil {
			return nil, fmt.Errorf("Reading secret failed: %s", err)
		}
		envvar := mesos.Environment_Variable{Name: k, Value: &secret}
		env.Variables = append(env.Variables, envvar)
	}
	var containerInfo mesos.ContainerInfo
	switch job.Container.Type {
	case model.Docker:
		containerInfo = mesos.ContainerInfo{
			Type: mesos.ContainerInfo_DOCKER.Enum(),
			Docker: &mesos.ContainerInfo_DockerInfo{
				Image:          job.Container.Docker.Image,
				ForcePullImage: &job.Container.Docker.ForcePullImage,
			},
		}
	case model.Mesos:
		containerInfo = mesos.ContainerInfo{
			Type: mesos.ContainerInfo_MESOS.Enum(),
			Docker: &mesos.ContainerInfo_DockerInfo{
				Image: job.Container.Mesos.Image,
			},
		}
	default:
		return nil, fmt.Errorf("Unknown container type: %s", job.Container.Type)
	}
	labels := make([]mesos.Label, len(job.Labels))
	for k, v := range job.Labels {
		func(v string) {
			labels = append(labels, mesos.Label{Key: k, Value: &v})
		}(v)
	}
	task := mesos.TaskInfo{
		TaskID: mesos.TaskID{Value: tid},
		Name:   "Task " + tid,
		Command: &mesos.CommandInfo{
			Value:       proto.String(job.Cmd),
			Environment: &env,
			User:        proto.String(job.User),
			Shell:       proto.Bool(job.Shell),
			Arguments:   job.Arguments,
		},
		Container: &containerInfo,
		Labels:    &mesos.Labels{labels},
	}
	return &task, nil
}

// Stores information about single run of a job.
func (sched *Scheduler) addTaskHistory(status *mesos.TaskStatus, start time.Time, jid *model.JobID) {
	executorID := status.GetExecutorID().GetValue()
	agentID := status.GetAgentID().GetValue()
	frameworkID := sched.frameworkID()
	task := model.Task{
		Start:       start,
		End:         time.Now(),
		TaskID:      status.TaskID.GetValue(),
		ExecutorID:  executorID,
		AgentID:     agentID,
		FrameworkID: frameworkID,
		ExecutorURL: fmt.Sprintf("%s/#/agents/%s/frameworks/%s/executors/%s", sched.leaderURL(), agentID, frameworkID, executorID),
	}
	if status.GetState() != mesos.TASK_FINISHED {
		task.Message = status.GetMessage()
		task.Reason = status.GetReason().String()
		task.Source = status.GetSource().String()
	}
	err := sched.storage.AddTask(jid.Group, jid.Project, jid.ID, &task)
	if err != nil {
		log.Errorf("Error saving task: %s", err)
	}
}

type taskID struct {
	jid  *model.JobID
	uuid string
}

func (tid *taskID) String() string {
	return fmt.Sprintf("%s:%s", tid.jid.String(), tid.uuid)
}

func newTaskID(jid *model.JobID) (string, error) {
	u4, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	tid := taskID{jid: jid, uuid: u4.String()}
	return tid.String(), nil
}

func parseTaskID(tid string) (*model.JobID, error) {
	idx := strings.LastIndex(tid, ":")
	if idx == -1 {
		return nil, errors.New("Task ID separator not found")
	}
	return model.ParseJobID(tid[:idx])
}

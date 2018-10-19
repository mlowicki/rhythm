package mesos

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
	"github.com/mlowicki/rhythm/mesos/reconciliation"
	"github.com/mlowicki/rhythm/model"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	offersCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "offers",
		Help: "Number of received offers.",
	})
	taskStateUpdatesCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "task_state_updates",
		Help: "Task state updates.",
	}, []string{"state"})
)

func init() {
	prometheus.MustRegister(offersCount)
	prometheus.MustRegister(taskStateUpdatesCount)
}

func buildSubscribedEventHandler(fidStore store.Singleton, failoverTimeout time.Duration, onSuccess func(*scheduler.Event)) eventrules.Rule {
	return eventrules.New(
		controller.TrackSubscription(fidStore, failoverTimeout),
		func(ctx context.Context, e *scheduler.Event, err error, ch eventrules.Chain) (context.Context, *scheduler.Event, error) {
			if err == nil {
				onSuccess(e)
			}
			return ch(ctx, e, err)
		},
	)
}

func buildUpdateEventHandler(stor storage, cli calls.Caller, reconciler *reconciliation.Reconciliation, frameworkIDStore, leaderURLStore store.Singleton) eventrules.Rule {
	return controller.AckStatusUpdates(cli).AndThen().HandleF(func(ctx context.Context, e *scheduler.Event) error {
		status := e.GetUpdate().GetStatus()
		reconciler.HandleUpdate(e.GetUpdate())
		id, err := taskID2JobID(status.TaskID.Value)
		if err != nil {
			log.WithFields(log.Fields{
				"taskID": status.TaskID.Value,
			}).Errorf("Failed to get job ID from task ID: %s", err)
			return nil
		}
		state := status.GetState()
		log.Printf("Task state update: %s (%s)", id, state)
		chunks := strings.Split(id, ":")
		job, err := stor.GetJob(chunks[0], chunks[1], chunks[2])
		if err != nil {
			log.Printf("Failed to get job for task: %s", id)
			return nil
		}
		if job == nil {
			log.Printf("Update for unknown job: %s", id)
			return nil
		}
		switch state {
		case mesos.TASK_STAGING:
			job.State = model.STAGING
		case mesos.TASK_STARTING:
			job.State = model.STARTING
		case mesos.TASK_RUNNING:
			job.State = model.RUNNING
		case mesos.TASK_FINISHED:
			log.Printf("Task finished successfully: %s", status.TaskID.Value)
			job.TaskID = ""
			job.AgentID = ""
			job.State = model.IDLE
			taskStateUpdatesCount.WithLabelValues("finished").Inc()
		case mesos.TASK_LOST:
			/*
			 * 1. Reconciliation run gets running task A
			 * 2. Task A finishes successfuly
			 * 3. Reconciliation for task A sent
			 * 4. TASK_LOST is received which would mark job A as failed
			 *
			 * It's still a small window when it's possible = if handler for TASK_LOST
			 * read from storage before handler for e.g. TASK_FINISHED persisted update.
			 */
			if status.GetReason() == mesos.REASON_RECONCILIATION {
				if job.State == model.IDLE || job.State == model.FAILED {
					return nil
				}
			}
			fallthrough
		case mesos.TASK_FAILED:
			fallthrough
		case mesos.TASK_KILLED:
			fallthrough
		case mesos.TASK_ERROR:
			frameworkID := store.GetIgnoreErrors(frameworkIDStore)()
			leaderURL := store.GetIgnoreErrors(leaderURLStore)()
			handleFailedTask(job, &status, &frameworkID, &leaderURL)
		default:
			log.Panicf("Unknown state: %s", state)
		}
		err = stor.SaveJob(job)
		if err != nil {
			log.Errorf("Failed to save job while handling update: %s", err)
		}
		return nil
	})
}

func handleFailedTask(job *model.Job, status *mesos.TaskStatus, frameworkID, leaderURL *string) {
	msg := status.GetMessage()
	reason := status.GetReason().String()
	src := status.GetSource().String()
	log.Errorf("Task failed: %s (%s; %s; %s; %s)", job, status.GetState(), msg, reason, src)
	job.State = model.FAILED
	executorID := status.GetExecutorID().GetValue()
	agentID := status.GetAgentID().GetValue()
	job.LastFailedTask = model.FailedTask{
		Message:     msg,
		Reason:      reason,
		Source:      src,
		When:        time.Now(),
		TaskID:      status.TaskID.GetValue(),
		ExecutorID:  executorID,
		AgentID:     agentID,
		FrameworkID: *frameworkID,
		ExecutorURL: fmt.Sprintf("%s/#/agents/%s/frameworks/%s/executors/%s", *leaderURL, agentID, *frameworkID, executorID),
	}
	job.TaskID = ""
	job.AgentID = ""
	taskStateUpdatesCount.WithLabelValues("failed").Inc()
}

func buildOffersEventHandler(stor storage, cli calls.Caller, secr secrets) events.HandlerFunc {
	return func(ctx context.Context, e *scheduler.Event) error {
		offers := e.GetOffers().GetOffers()
		offersCount.Add(float64(len(offers)))
		log.Debugf("Received offers: %d", len(offers))
		js, err := stor.GetRunnableJobs()
		if err != nil {
			log.Errorf("Failed to get runnable jobs: %s", err)
			return nil
		}
		for i := range offers {
			js = handleOffer(ctx, cli, &offers[i], js, secr, stor)
		}
		return nil
	}
}

func logAllEvents() eventrules.Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, ch eventrules.Chain) (context.Context, *scheduler.Event, error) {
		log.Debugf("%+v", *e)
		return ch(ctx, e, err)
	}
}

func taskID2JobID(id string) (string, error) {
	idx := strings.LastIndexByte(id, ':')
	if idx == -1 {
		return "", errors.New("Separator not found")
	}
	return id[:strings.LastIndexByte(id, ':')], nil
}

func handleOffer(ctx context.Context, cli calls.Caller, off *mesos.Offer, jobs []*model.Job, secr secrets, stor storage) []*model.Job {
	tasks := []mesos.TaskInfo{}
	resLeft := mesos.Resources(off.Resources).ToUnreserved().Unallocate()
	var jobsLeft, jobsUsed []*model.Job
	for _, job := range jobs {
		res := mesos.Resources{}
		res.Add(
			resources.NewCPUs(job.CPUs).Resource,
			resources.NewMemory(job.Mem).Resource,
		)
		if !resources.ContainsAll(resLeft, res) {
			jobsLeft = append(jobsLeft, job)
			continue
		}
		err, task := newTaskInfo(job, secr)
		if err != nil {
			log.Errorf("Failed to create task info: %s", err)
			continue
		}
		task.AgentID = off.AgentID
		task.Resources = resources.Find(res, resLeft...)
		if task.Resources == nil {
			log.Fatal("Resources not found")
		}
		resLeft.Subtract(task.Resources...)
		tasks = append(tasks, *task)
		jobsUsed = append(jobsUsed, job)
	}
	accept := calls.Accept(calls.OfferOperations{calls.OpLaunch(tasks...)}.WithOffers(off.ID))
	err := calls.CallNoData(ctx, cli, accept.With(calls.RefuseSeconds(time.Hour)))
	if err != nil {
		log.Errorf("Failed to accept offer: %s", err)
		return nil
	}
	for i, job := range jobsUsed {
		job.State = model.STAGING
		job.LastStartAt = time.Now()
		job.TaskID = tasks[i].TaskID.GetValue()
		job.AgentID = tasks[i].AgentID.GetValue()
		err := stor.SaveJob(job)
		if err != nil {
			log.Errorf("Failed to update job after accepting offer: %s", err)
		}
		log.Printf("Job staged: %s", job)
		taskStateUpdatesCount.WithLabelValues("staged").Inc()
	}
	return jobsLeft
}

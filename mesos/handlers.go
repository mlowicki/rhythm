package mesos

import (
	"context"
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
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	log "github.com/sirupsen/logrus"
)

func buildEventHandler(client calls.Caller, frameworkID store.Singleton, secr secrets, stor storage, c *conf.Conf) events.Handler {
	logger := controller.LogEvents(func(e *scheduler.Event) {
		log.Printf("Event: %s", e)
	}).Unless(c.Verbose)
	return eventrules.New(
		logAllEvents().If(c.Verbose),
		controller.LiftErrors(),
	).Handle(events.Handlers{
		scheduler.Event_SUBSCRIBED: buildSubscribedEventHandler(frameworkID, c.Mesos.FailoverTimeout),
		scheduler.Event_OFFERS:     buildOffersEventHandler(stor, client, secr),
		scheduler.Event_UPDATE:     buildUpdateEventHandler(stor, client),
	}.Otherwise(logger.HandleEvent))
}

func buildSubscribedEventHandler(fidStore store.Singleton, failoverTimeout time.Duration) eventrules.Rule {
	return eventrules.New(controller.TrackSubscription(fidStore, failoverTimeout))
}

func buildUpdateEventHandler(stor storage, cli calls.Caller) eventrules.Rule {
	return controller.AckStatusUpdates(cli).AndThen().HandleF(func(ctx context.Context, e *scheduler.Event) error {
		status := e.GetUpdate().GetStatus()
		id := taskID2JobID(status.TaskID.Value)
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
			job.State = model.IDLE
		case mesos.TASK_FAILED:
			fallthrough
		case mesos.TASK_KILLED:
			fallthrough
		case mesos.TASK_ERROR:
			fallthrough
		case mesos.TASK_LOST:
			handleFailedTask(job, &status)
		default:
			log.Panicf("Unknown state: %s", state)
		}
		err = stor.SaveJob(job)
		if err != nil {
			log.Printf("Failed to save job while handling update: %s", err)
		}
		return nil
	})
}

func handleFailedTask(job *model.Job, status *mesos.TaskStatus) {
	msg := status.GetMessage()
	reason := status.GetReason().String()
	src := status.GetSource().String()
	log.Printf("Task failed: %s (%s; %s; %s; %s)", job, status.GetState(), msg, reason, src)
	job.State = model.FAILED
	job.LastFail = model.LastFail{
		Message: msg,
		Reason:  reason,
		Source:  src,
		When:    time.Now(),
	}
}

func buildOffersEventHandler(stor storage, cli calls.Caller, secr secrets) events.HandlerFunc {
	return func(ctx context.Context, e *scheduler.Event) error {
		offers := e.GetOffers().GetOffers()
		log.Printf("Received offers: %d", len(offers))
		js, err := stor.GetRunnableJobs()
		if err != nil {
			log.Printf("Failed to get runnable jobs: %s", err)
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
		log.Printf("%+v", *e)
		return ch(ctx, e, err)
	}
}

func taskID2JobID(id string) string {
	return id[:strings.LastIndexByte(id, ':')]
}

// TODO Handle reservations
func handleOffer(ctx context.Context, cli calls.Caller, off *mesos.Offer, js []*model.Job, secr secrets, stor storage) []*model.Job {
	tasks := []mesos.TaskInfo{}
	resLeft := mesos.Resources(off.Resources)
	var jsLeft, jsUsed []*model.Job
	for _, j := range js {
		res := mesos.Resources{}
		res.Add(
			resources.NewCPUs(j.CPUs).Resource,
			resources.NewMemory(j.Mem).Resource,
		)
		if !resources.ContainsAll(resLeft.ToUnreserved(), res) {
			jsLeft = append(jsLeft, j)
			continue
		}
		err, task := newTaskInfo(j, secr)
		if err != nil {
			log.Printf("Failed to create task info: %s", err)
			continue
		}
		task.AgentID = off.AgentID
		task.Resources = resources.Find(res, resLeft...)
		resLeft.Subtract(task.Resources...)
		tasks = append(tasks, *task)
		jsUsed = append(jsUsed, j)
	}
	err := calls.CallNoData(ctx, cli,
		calls.Accept(calls.OfferOperations{calls.OpLaunch(tasks...)}.WithOffers(off.ID)))
	if err != nil {
		log.Printf("Failed to accept offer: %s", err)
		return nil
	}
	for _, j := range jsUsed {
		j.State = model.STAGING
		j.LastStartAt = time.Now()
		err := stor.SaveJob(j)
		if err != nil {
			log.Printf("Failed to update job after accepting offer: %s", err)
		}
		log.Printf("Job launched: %s", j)
	}
	return jsLeft
}

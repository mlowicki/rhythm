package mesos

import (
	"context"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
	"github.com/mlowicki/rhythm/mesos/jobsscheduler"
	"github.com/mlowicki/rhythm/mesos/reconciliation"
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

func buildUpdateEventHandler(cli calls.Caller, reconciler *reconciliation.Reconciliation, jobsSched *jobsscheduler.Scheduler) eventrules.Rule {
	var h eventrules.Rule
	h = func(ctx context.Context, e *scheduler.Event, err error, ch eventrules.Chain) (context.Context, *scheduler.Event, error) {
		status := e.GetUpdate().GetStatus()
		jobsSched.HandleTaskStateUpdate(&status)
		reconciler.HandleTaskStateUpdate(&status)
		switch status.GetState() {
		case mesos.TASK_FINISHED:
			taskStateUpdatesCount.WithLabelValues("finished").Inc()
		case mesos.TASK_LOST:
			fallthrough
		case mesos.TASK_FAILED:
			fallthrough
		case mesos.TASK_KILLED:
			fallthrough
		case mesos.TASK_ERROR:
			taskStateUpdatesCount.WithLabelValues("failed").Inc()
		}
		return ch(ctx, e, err)
	}
	return h.AndThen(controller.AckStatusUpdates(cli))
}


func buildOffersEventHandler(cli calls.Caller, jobsSched *jobsscheduler.Scheduler) events.HandlerFunc {
	return func(ctx context.Context, e *scheduler.Event) error {
		offers := e.GetOffers().GetOffers()
		offersCount.Add(float64(len(offers)))
		log.Debugf("Number of received offers: %d", len(offers))
		for i := range offers {
			if ctx.Err() != nil {
				break
			}
			offer := offers[i]
			tasks := jobsSched.FindTasksForOffer(ctx, &offer)
			accept := calls.Accept(calls.OfferOperations{calls.OpLaunch(tasks...)}.WithOffers(offer.ID))
			err := calls.CallNoData(ctx, cli, accept.With(calls.RefuseSeconds(time.Hour)))
			if err != nil {
				log.Errorf("Failed to accept offer: %s", err)
				return nil
			} else {
				for _, task := range tasks {
					log.Debugf("Task staged: %s", task.TaskID.Value)
				}
			}
			taskStateUpdatesCount.WithLabelValues("staged").Add(float64(len(tasks)))
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

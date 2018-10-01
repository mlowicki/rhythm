package mesos

import (
	"context"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/mesos/offerstuner"
	"github.com/mlowicki/rhythm/mesos/reconciliation"
	log "github.com/sirupsen/logrus"
)

var (
	registrationMinBackoff = 1 * time.Second
	registrationMaxBackoff = 15 * time.Second
)

func Run(c *conf.Conf, ctx context.Context, stor storage, secr secrets) error {
	frameworkID, err := newFrameworkIDStore(stor)
	if err != nil {
		return err
	}
	cli, err := newClient(&c.Mesos, frameworkID)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	rec := reconciliation.New(ctx, cli, stor)
	tun := offerstuner.New(ctx, cli, stor)
	logger := controller.LogEvents(func(e *scheduler.Event) {
		log.Printf("Event: %s", e)
	}).Unless(c.Mesos.LogAllEvents)
	handler := eventrules.New(
		logAllEvents().If(c.Mesos.LogAllEvents),
		controller.LiftErrors(),
	).Handle(events.Handlers{
		scheduler.Event_HEARTBEAT: events.HandlerFunc(func(ctx context.Context, e *scheduler.Event) error {
			log.Debug("Heartbeat")
			return nil
		}),
		scheduler.Event_ERROR: events.HandlerFunc(func(ctx context.Context, e *scheduler.Event) error {
			log.Error(e.GetError().Message)
			return nil
		}),
		scheduler.Event_SUBSCRIBED: buildSubscribedEventHandler(frameworkID, c.Mesos.FailoverTimeout, func() {
			rec.Run()
			tun.Run()
		}),
		scheduler.Event_OFFERS: buildOffersEventHandler(stor, cli, secr),
		scheduler.Event_UPDATE: buildUpdateEventHandler(stor, cli, rec),
	}.Otherwise(logger.HandleEvent))
	controller.Run(
		ctx,
		newFrameworkInfo(&c.Mesos, frameworkID),
		cli,
		controller.WithRegistrationTokens(
			backoff.Notifier(registrationMinBackoff, registrationMaxBackoff, ctx.Done()),
		),
		controller.WithEventHandler(handler),
		controller.WithSubscriptionTerminated(func(err error) {
			log.Printf("Connection to Mesos terminated: %v\n", err)
			if err != nil && err.Error() == "Framework has been removed" {
				log.Println("Resetting framework ID")
				if err := frameworkID.Set(""); err != nil {
					log.Fatal(err)
				}
				cancel()
			}
		}),
	)
	return nil
}

package mesos

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
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

func getLeaderHost(info *mesos.MasterInfo) string {
	addr := info.GetAddress()
	host := *addr.Hostname
	if host == "" {
		host = *addr.IP
	}
	if addr.Port != 0 {
		host += fmt.Sprintf(":%d", addr.Port)
	}
	return host
}

func Run(c *conf.Conf, ctx context.Context, stor storage, secr secrets) error {
	frameworkIDStore, err := newFrameworkIDStore(stor)
	if err != nil {
		return err
	}
	leaderURLStore := store.NewInMemorySingleton()
	cli, err := newClient(&c.Mesos, frameworkIDStore)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(ctx)
	reconciler := reconciliation.New(ctx, cli, stor)
	offersTuner := offerstuner.New(ctx, cli, stor)
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
		scheduler.Event_SUBSCRIBED: buildSubscribedEventHandler(frameworkIDStore, c.Mesos.FailoverTimeout, func(e *scheduler.Event) {
			firstMesosURL, err := url.Parse(c.Mesos.Addrs[0])
			scheme := "https"
			if err != nil {
				log.Error(err)
			} else {
				scheme = firstMesosURL.Scheme
			}
			leaderHost := getLeaderHost(e.GetSubscribed().GetMasterInfo())
			leaderURL := url.URL{Scheme: scheme, Host: leaderHost}
			log.Infof("Leading master URL: %s", &leaderURL)
			leaderURLStore.Set(leaderURL.String())
			reconciler.Run()
			offersTuner.Run()
		}),
		scheduler.Event_OFFERS: buildOffersEventHandler(stor, cli, secr),
		scheduler.Event_UPDATE: buildUpdateEventHandler(stor, cli, reconciler, frameworkIDStore, leaderURLStore),
	}.Otherwise(logger.HandleEvent))
	controller.Run(
		ctx,
		newFrameworkInfo(&c.Mesos, frameworkIDStore),
		cli,
		controller.WithRegistrationTokens(
			backoff.Notifier(registrationMinBackoff, registrationMaxBackoff, ctx.Done()),
		),
		controller.WithEventHandler(handler),
		controller.WithSubscriptionTerminated(func(err error) {
			log.Printf("Connection to Mesos terminated: %v\n", err)
			if err != nil && err.Error() == "Framework has been removed" {
				log.Println("Resetting framework ID")
				if err := frameworkIDStore.Set(""); err != nil {
					log.Fatal(err)
				}
				cancel()
			}
		}),
	)
	return nil
}

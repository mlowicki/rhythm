package mesos

import (
	"context"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mlowicki/rhythm/conf"
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
	controller.Run(
		ctx,
		newFrameworkInfo(&c.Mesos, frameworkID),
		cli,
		controller.WithRegistrationTokens(
			backoff.Notifier(registrationMinBackoff, registrationMaxBackoff, ctx.Done()),
		),
		controller.WithEventHandler(buildEventHandler(cli, frameworkID, secr, stor, c, rec)),
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

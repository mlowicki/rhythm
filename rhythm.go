package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mlowicki/rhythm/api"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/coordinator"
	"github.com/mlowicki/rhythm/mesos"
	"github.com/mlowicki/rhythm/secrets"
	"github.com/mlowicki/rhythm/storage"
)

var (
	registrationMinBackoff = 1 * time.Second
	registrationMaxBackoff = 15 * time.Second
)

func buildConf() *conf.Conf {
	confPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	var conf, err = conf.New(*confPath)
	if err != nil {
		log.Fatalf("Error getting configuration: %s\n", err)
	}
	return conf
}

/* TODO Periodic reconciliation

reconcile := calls.Reconcile(calls.ReconcileTasks(nil))
resp, err := cli.Call(context.TODO(), reconcile)
if err != nil {
	log.Fatal(err)
}
log.Printf("response: %#v\n", resp)

*/
// TODO Configure ACLs for ZooKeeper
func main() {
	conf := buildConf()
	stor := storage.New(&conf.Storage)
	coord := coordinator.New(&conf.Coordinator)
	api.New(&conf.API, stor)
	vaultC := secrets.New(&conf.Secrets)
	for {
		frameworkIDStore, err := mesos.NewFrameworkIDStore(stor)
		if err != nil {
			log.Printf("Failed getting framework ID store: %s\n", err)
			<-time.After(time.Second)
			continue
		}
		ctx, err := coord.WaitUntilLeader()
		if err != nil {
			log.Printf("Error waiting for being a leader: %s\n", err)
			<-time.After(time.Second)
			continue
		}
		ctx, cancel := context.WithCancel(ctx)
		mesosC := mesos.NewClient(&conf.Mesos, frameworkIDStore)
		controller.Run(
			ctx,
			mesos.NewFrameworkInfo(&conf.Mesos, frameworkIDStore),
			mesosC,
			controller.WithRegistrationTokens(
				backoff.Notifier(registrationMinBackoff, registrationMaxBackoff, ctx.Done()),
			),
			controller.WithEventHandler(mesos.BuildEventHandler(mesosC, frameworkIDStore, vaultC, stor, conf.Verbose)),
			controller.WithSubscriptionTerminated(func(err error) {
				log.Printf("Connection to Mesos terminated: %v\n", err)
				if err.Error() == "Framework has been removed" {
					log.Println("Resetting framework ID")
					if err := frameworkIDStore.Set(""); err != nil {
						log.Fatal(err)
					}
					cancel()
				}
			}),
		)
	}
}

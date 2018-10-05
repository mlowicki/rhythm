package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/mlowicki/rhythm/api"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/coordinator"
	"github.com/mlowicki/rhythm/mesos"
	"github.com/mlowicki/rhythm/secrets"
	"github.com/mlowicki/rhythm/storage"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var leaderGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "leader",
	Help: "Indicates if instance is elected as leader.",
})

func init() {
	prometheus.MustRegister(leaderGauge)
}

func main() {
	confPath := flag.String("config", "config.json", "Path to configuration file")
	testLogging := flag.Bool("testlogging", false, "")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *version {
		fmt.Println("0.1")
		return
	}
	var conf, err = conf.New(*confPath)
	if err != nil {
		log.Fatalf("Error getting configuration: %s", err)
	}
	initLogging(&conf.Logging)
	if *testLogging {
		log.Error("test")
		log.Info("Sending test event. Wait 10s...")
		<-time.After(10 * time.Second)
		return
	}
	stor := storage.New(&conf.Storage)
	coord := coordinator.New(&conf.Coordinator)
	api.New(&conf.API, stor)
	secr := secrets.New(&conf.Secrets)
	for {
		ctx, err := coord.WaitUntilLeader()
		if err != nil {
			log.Errorf("Error waiting for being a leader: %s", err)
			<-time.After(time.Second)
			continue
		}
		leaderGauge.Set(1)
		err = mesos.Run(conf, ctx, stor, secr)
		leaderGauge.Set(0)
		if err != nil {
			log.Errorf("Controller error: %s", err)
			<-time.After(time.Second)
		}
	}
}

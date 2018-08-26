package main

import (
	"flag"
	"time"

	"github.com/mlowicki/rhythm/api"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/coordinator"
	"github.com/mlowicki/rhythm/mesos"
	"github.com/mlowicki/rhythm/secrets"
	"github.com/mlowicki/rhythm/storage"
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.AddHook(filename.NewHook())
}

func buildConf() *conf.Conf {
	confPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	var conf, err = conf.New(*confPath)
	if err != nil {
		log.Fatalf("Error getting configuration: %s\n", err)
	}
	return conf
}

// TODO Configure ACLs for ZooKeeper
func main() {
	conf := buildConf()
	stor := storage.New(&conf.Storage)
	coord := coordinator.New(&conf.Coordinator)
	api.New(&conf.API, stor)
	secr := secrets.New(&conf.Secrets)
	for {
		ctx, err := coord.WaitUntilLeader()
		if err != nil {
			log.Printf("Error waiting for being a leader: %s\n", err)
			<-time.After(time.Second)
			continue
		}
		err = mesos.Run(conf, ctx, stor, secr)
		if err != nil {
			log.Printf("Controller error: %s\n", err)
			<-time.After(time.Second)
		}
	}
}

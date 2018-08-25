package coordinator

import (
	"context"
	"log"

	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/coordinator/zk"
)

type coordinator interface {
	WaitUntilLeader() (context.Context, error)
}

func New(c *conf.Coordinator) coordinator {
	if c.Type == conf.CoordinatorTypeZooKeeper {
		coord, err := zk.New(&c.ZooKeeper)
		if err != nil {
			log.Fatalf("Error creating coordinator: %s\n", err)
		}
		return coord
	} else {
		log.Fatalf("Unknown coordinator type: %s\n", c.Type)
		return nil
	}
}

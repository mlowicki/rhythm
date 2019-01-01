package coordinator

import (
	"context"

	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/coordinator/zk"
	log "github.com/sirupsen/logrus"
)

type coordinator interface {
	WaitUntilLeader() context.Context
}

// New creates fresh coordinator instance.
func New(c *conf.Coordinator) coordinator {
	if c.Backend == conf.CoordinatorBackendZK {
		coord, err := zk.New(&c.ZooKeeper)
		if err != nil {
			log.Fatal(err)
		}
		return coord
	}
	log.Fatalf("Unknown backend: %s", c.Backend)
	return nil
}

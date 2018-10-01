package offerstuner

// The goal of offers tunning is to follow multi-scheduler scalability -
// https://mesos.apache.org/documentation/latest/app-framework-development-guide/#multi-scheduler-scalability.
// Offers tuner calls SUPPRESS when there is no tasks to run in nearest future or
// calls REVIVE when there is at least one delayed task waiting for offer.

import (
	"context"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mlowicki/rhythm/model"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	suppressCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "offerstuner_suppress_calls",
		Help: "Number of suppress offers calls.",
	})
	reviveCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "offerstuner_revive_calls",
		Help: "Number of revive offers calls.",
	})
	roundsCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "offerstuner_rounds",
		Help: "Number of rounds.",
	})
)

func init() {
	prometheus.MustRegister(suppressCount)
	prometheus.MustRegister(reviveCount)
	prometheus.MustRegister(roundsCount)
}

type storage interface {
	GetJobs() ([]*model.Job, error)
}

func findMaxDelay(jobs []*model.Job) time.Duration {
	var maxDelay time.Duration
	now := time.Now()
	for _, job := range jobs {
		next := job.NextRun()
		delay := now.Sub(next)
		if delay > maxDelay {
			maxDelay = delay
		}
	}
	return maxDelay
}

func findMinDeadline(jobs []*model.Job) time.Duration {
	now := time.Now()
	minDeadline := time.Hour * 24
	for _, job := range jobs {
		next := job.NextRun()
		delay := next.Sub(now)
		if delay >= 0 && delay < minDeadline {
			minDeadline = delay
		}
	}
	return minDeadline
}

type Tuner struct {
	ctx  context.Context
	cli  calls.Caller
	stor storage
}

func (t *Tuner) round(reviveTokens <-chan struct{}, suppressed bool) (bool, error) {
	jobs, err := t.stor.GetJobs()
	if err != nil {
		return suppressed, err
	}
	maxDelay := findMaxDelay(jobs)
	minDeadline := findMinDeadline(jobs)
	log.Debugf("Max delay: %s", maxDelay)
	log.Debugf("Min deadline: %s", minDeadline)
	if (maxDelay > time.Minute) || (suppressed && (maxDelay > 0 || minDeadline < time.Minute)) {
		select {
		case <-reviveTokens:
			err := calls.CallNoData(t.ctx, t.cli, calls.Revive())
			if err != nil {
				return suppressed, err
			} else {
				reviveCount.Inc()
				suppressed = false
				log.Debug("Revived offers")
			}
		default:
		}
	} else if maxDelay == 0 && minDeadline > time.Minute && !suppressed {
		err := calls.CallNoData(t.ctx, t.cli, calls.Suppress())
		if err != nil {
			return suppressed, err
		} else {
			suppressCount.Inc()
			suppressed = true
			log.Debug("Suppressed offers")
		}
	}
	return suppressed, nil
}

func (t *Tuner) Run() {
	go func() {
		log.Println("Started")
		for {
			err := calls.CallNoData(t.ctx, t.cli, calls.Revive())
			if err != nil {
				log.Errorf("Failed to revive offers: %s. Retry in 10s.", err)
				<-time.After(time.Second * 10)
			} else {
				reviveCount.Inc()
				log.Debug("Revived offers")
				break
			}
		}
		reviveCount.Inc()
		reviveTokens := backoff.BurstNotifier(1, time.Minute, time.Minute, t.ctx.Done())
		suppressed := false
		for {
			select {
			case <-t.ctx.Done():
				log.Println("Terminated")
				return
			case <-time.After(time.Minute):
				var err error
				suppressed, err = t.round(reviveTokens, suppressed)
				if err != nil {
					log.Error(err)
				}
				roundsCount.Inc()
			}
		}
	}()
}

func New(ctx context.Context, cli calls.Caller, stor storage) *Tuner {
	tuner := Tuner{
		ctx:  ctx,
		cli:  cli,
		stor: stor,
	}
	return &tuner
}

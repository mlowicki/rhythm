package reconciliation

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mlowicki/rhythm/model"
	log "github.com/sirupsen/logrus"
)

const (
	roundInterval           = time.Minute * 10
	roundRetry              = time.Second * 10
	initialReconcileTimeout = time.Second * 4
)

type Reconciliation struct {
	ctx       context.Context
	cli       calls.Caller
	storage   storage
	roundQ    chan struct{}
	updatesCh chan string
	running   bool
	mut       sync.Mutex
}

type storage interface {
	GetJobs() ([]*model.Job, error)
}

func (rec *Reconciliation) HandleTaskStateUpdate(status *mesos.TaskStatus) {
	if status.GetReason() != mesos.REASON_RECONCILIATION {
		return
	}
	select {
	case rec.updatesCh <- status.TaskID.Value:
	default:
	}
}

func (rec *Reconciliation) round() error {
	jobs, err := rec.storage.GetJobs()
	if err != nil {
		return err
	}
	tasks := make(map[string]string)
	for _, job := range jobs {
		if job.CurrentTaskID != "" {
			tasks[job.CurrentTaskID] = job.CurrentAgentID
		}
	}
	boff := backoff.NewExponentialBackOff()
	boff.InitialInterval = initialReconcileTimeout
	boff.MaxElapsedTime = 0
	ticker := backoff.NewTicker(boff)
	<-ticker.C // Ticker is guaranteed to tick at least once.
	defer ticker.Stop()
	for len(tasks) > 0 {
		_, err := rec.cli.Call(rec.ctx, calls.Reconcile(calls.ReconcileTasks(tasks)))
		if err != nil {
			return err
		}
	inner:
		for {
			select {
			case <-rec.ctx.Done():
				return nil
			case <-ticker.C:
				break inner
			case taskID := <-rec.updatesCh:
				delete(tasks, taskID)
				if len(tasks) == 0 {
					break inner
				}
			}
		}
	}
	return nil
}

func (rec *Reconciliation) queueRound() {
	select {
	case rec.roundQ <- struct{}{}:
	default:
	}
}

func (rec *Reconciliation) Run() {
	rec.mut.Lock()
	if !rec.running {
		go func() {
			timer := time.After(roundInterval)
			for {
				select {
				case <-rec.ctx.Done():
					rec.mut.Lock()
					rec.running = false
					rec.mut.Unlock()
					return
				case <-timer:
					rec.queueRound()
				case <-rec.roundQ:
					log.Debug("Round started")
					err := rec.round()
					if err != nil {
						log.Errorf("Round failed: %s", err)
						timer = time.After(roundRetry)
					} else {
						log.Debug("Round finished")
						timer = time.After(roundInterval)
					}
				}
			}
		}()
		rec.running = true
	}
	rec.mut.Unlock()
	rec.queueRound()
}

func New(ctx context.Context, cli calls.Caller, stor storage) *Reconciliation {
	rec := &Reconciliation{
		ctx:       ctx,
		cli:       cli,
		roundQ:    make(chan struct{}, 1),
		storage:   stor,
		updatesCh: make(chan string),
	}
	return rec
}

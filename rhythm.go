package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/gogo/protobuf/proto"
	vault "github.com/hashicorp/vault/api"
	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/callrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/controller"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/extras/store"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli/httpsched"
	"github.com/mesos/mesos-go/api/v1/lib/resources"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
	"github.com/mlowicki/rhythm/api"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/election"
	mes "github.com/mlowicki/rhythm/mesos"
	"github.com/mlowicki/rhythm/model"
	"github.com/mlowicki/rhythm/storage/zk"
)

var (
	registrationMinBackoff = 1 * time.Second
	registrationMaxBackoff = 15 * time.Second
)

func getConf() *conf.Conf {
	confPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()
	var conf, err = conf.NewConf(*confPath)
	if err != nil {
		log.Fatalf("Error getting configuration: %s\n", err)
	}
	return conf
}

func getStorage(c *conf.ZooKeeper) *zk.ZKStorage {
	s, err := zk.NewStorage(c)
	if err != nil {
		log.Fatalf("Error initializing storage: %s\n", err)
	}
	return s
}

func getElection(c *conf.ZooKeeper) *election.Election {
	elec, err := election.New(c)
	if err != nil {
		log.Fatalf("Error creating election: %s\n", err)
	}
	return elec
}

func getVaultClient(c *conf.Vault) *vault.Client {
	url, err := url.Parse(c.Address)
	if err != nil {
		log.Fatalf("Error parsing Vault address: %s\n", err)
	}
	if url.Scheme != "https" {
		log.Printf("Vault address uses HTTP scheme which is insecure. It's recommented to use HTTPS instead.")
	}
	cli, err := vault.NewClient(&vault.Config{
		Address: c.Address,
		Timeout: c.Timeout,
	})
	if err != nil {
		log.Fatalf("Error creating Vault client: %s\n", err)
	}
	cli.SetToken(c.Token)
	return cli
}

func getMesosClient(c *conf.Mesos, frameworkIDStore store.Singleton) calls.Caller {
	return callrules.New(
		logCalls(map[scheduler.Call_Type]string{scheduler.Call_SUBSCRIBE: "Connecting to Mesos..."}),
		callrules.WithFrameworkID(store.GetIgnoreErrors(frameworkIDStore)),
	).Caller(httpsched.NewCaller(mes.NewHTTPClient(c), httpsched.AllowReconnection(true)))
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
	conf := getConf()
	storage := getStorage(&conf.ZooKeeper)
	api.NewAPI(&conf.API, storage)
	vaultC := getVaultClient(&conf.Vault)
	elec := getElection(&conf.ZooKeeper)
	for {
		frameworkIDStore, err := mes.NewFrameworkIDStore(storage)
		if err != nil {
			log.Printf("Failed getting framework ID store: %s\n", err)
			<-time.After(time.Second)
			continue
		}
		ctx, err := elec.WaitUntilLeader()
		if err != nil {
			log.Printf("Error waiting for being a leader: %s\n", err)
			<-time.After(time.Second)
			continue
		}
		ctx, cancel := context.WithCancel(ctx)
		mesosC := getMesosClient(&conf.Mesos, frameworkIDStore)
		controller.Run(
			ctx,
			mes.NewFrameworkInfo(&conf.Mesos, frameworkIDStore),
			mesosC,
			controller.WithRegistrationTokens(
				backoff.Notifier(registrationMinBackoff, registrationMaxBackoff, ctx.Done()),
			),
			controller.WithEventHandler(buildEventHandler(mesosC, frameworkIDStore, vaultC, storage, conf.Verbose)),
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

func logCalls(messages map[scheduler.Call_Type]string) callrules.Rule {
	return func(ctx context.Context, c *scheduler.Call, r mesos.Response, err error, ch callrules.Chain) (context.Context, *scheduler.Call, mesos.Response, error) {
		if message, ok := messages[c.GetType()]; ok {
			log.Println(message)
		}
		return ch(ctx, c, r, err)
	}
}

func handleOffer(ctx context.Context, cli calls.Caller, offer *mesos.Offer, jobs []*model.Job, vaultC *vault.Client, s Storage) []*model.Job {
	var jobsToLaunch []*model.Job
	tasks := []mesos.TaskInfo{}
	// TODO Handle reservations
	remaining := mesos.Resources(offer.Resources)
	if len(jobs) == 0 {
		goto accept
	}
	for _, job := range jobs {
		rs := mesos.Resources{}
		rs.Add(
			resources.NewCPUs(job.CPUs).Resource,
			resources.NewMemory(job.Mem).Resource,
		)
		flattened := remaining.ToUnreserved()
		if resources.ContainsAll(flattened, rs) {
			foundRs := resources.Find(rs, remaining...)
			u4, err := uuid.NewV4()
			if err != nil {
				log.Printf("Failed to generate UUID for task: %s\n", err)
				continue
			}
			taskID := fmt.Sprintf("%s:%s:%s:%s", job.Group, job.Project, job.ID, u4)
			env := mesos.Environment{
				Variables: []mesos.Environment_Variable{
					{Name: "TASK_ID", Value: &taskID},
					{
						Name: "secret",
						Type: mesos.Environment_Variable_SECRET.Enum(),
						Secret: &mesos.Secret{
							Type:  *mesos.Secret_VALUE.Enum(),
							Value: &mesos.Secret_Value{Data: []byte("secret")},
						},
					},
				},
			}
			for k, v := range job.Env {
				env.Variables = append(env.Variables, mesos.Environment_Variable{Name: k, Value: func(v string) *string { return &v }(v)})
			}

			/*
				secret, err := vaultC.Logical().Read("secret/bar")
				if err != nil {
					panic(err)
				}
				if secret == nil {
					panic("secret not found")
				}

				if value, ok := secret.Data["value"]; ok {
					switch v := value.(type) {
					case string:
						env.Variables = append(env.Variables, mesos.Environment_Variable{Name: "BAR", Value: &v})
					default:
						panic("secret is not a string")
					}
				}
			*/

			if job.Container.Kind != model.Docker { // TODO
				panic("Only Docker containers are supported")
			}
			task := mesos.TaskInfo{
				TaskID:    mesos.TaskID{Value: taskID},
				AgentID:   offer.AgentID,
				Resources: foundRs,
				Command: &mesos.CommandInfo{
					Value:       proto.String(job.Cmd), // TODO Cmd should be optional
					Environment: &env,
					// TODO Make 'Shell' configurable
					User: func(u string) *string { return &u }(job.User),
				},
				Container: &mesos.ContainerInfo{
					Type: mesos.ContainerInfo_DOCKER.Enum(),
					Docker: &mesos.ContainerInfo_DockerInfo{
						Image: job.Container.Docker.Image,
					},
				},
			}

			task.Name = "Task " + task.TaskID.Value
			tasks = append(tasks, task)
			remaining.Subtract(task.Resources...)
			jobsToLaunch = append(jobsToLaunch, job)
		}
	}
accept:
	accept := calls.Accept(
		calls.OfferOperations{calls.OpLaunch(tasks...)}.WithOffers(offer.ID),
	)
	err := calls.CallNoData(ctx, cli, accept)
	if err != nil {
		log.Printf("Failed to launch tasks: %s\n", err)
		return nil
	} else {
		for _, job := range jobsToLaunch {
			job.State = model.RUNNING
			job.LastStartAt = time.Now()
			err := s.SaveJob(job)
			if err != nil {
				log.Printf("Failed to save job while handling offer: %s\n", err)
			}
			log.Printf("Job launched: %s\n", job)
		}
		left := make([]*model.Job, len(jobs)-len(jobsToLaunch))
		contains := func(js []*model.Job, j *model.Job) bool {
			for _, c := range js {
				if c.Group == j.Group && c.Project == j.Project && c.ID == j.ID {
					return true
				}
			}
			return false
		}
		for _, j := range jobs {
			if !contains(jobsToLaunch, j) {
				left = append(left, j)
			}
		}
		return left
	}
}

func taskID2JobID(id string) string {
	return id[:strings.LastIndexByte(id, ':')]
}

func buildEventHandler(cli calls.Caller, fidStore store.Singleton, vaultC *vault.Client, s Storage, verbose bool) events.Handler {
	logger := controller.LogEvents(nil).Unless(false)
	return eventrules.New(
		logAllEvents().If(verbose),
		controller.LiftErrors(),
	).Handle(events.Handlers{
		scheduler.Event_HEARTBEAT: eventrules.HandleF(func(ctx context.Context, e *scheduler.Event) error {
			log.Println("Heartbeat")
			return nil
		}),
		scheduler.Event_UPDATE: controller.AckStatusUpdates(cli).AndThen().HandleF(func(ctx context.Context, e *scheduler.Event) error {
			status := e.GetUpdate().GetStatus()
			id := taskID2JobID(status.TaskID.Value)
			chunks := strings.Split(id, ":")
			job, err := s.GetJob(chunks[0], chunks[1], chunks[2])
			if err != nil {
				log.Printf("Failed to get job for task: %s\n", id)
				return nil
			}
			if job == nil {
				log.Printf("Update for unknown job: %s\n", id)
				return nil
			}
			// TODO Handle all states (https://github.com/mesos/mesos-go/blob/master/api/v1/lib/mesos.proto#L2212).
			switch state := status.GetState(); state {
			case mesos.TASK_STARTING:
				job.State = model.STARTING
			case mesos.TASK_RUNNING:
				job.State = model.RUNNING
			case mesos.TASK_FINISHED:
				log.Printf("Task finished: %s\n", status.TaskID.Value)
				job.State = model.IDLE
			case mesos.TASK_FAILED:
				// TODO Store last error(s) in job.
				log.Printf("Task '%s' failed: %s (reason: %s, source: %s)\n", id, status.GetMessage(), status.GetReason(), status.GetSource())
				job.State = model.FAILED
			case mesos.TASK_LOST:
				log.Printf("Task '%s' lost: %s (reason: %s, source: %s)\n", id, status.GetMessage(), status.GetReason(), status.GetSource())
				job.State = model.FAILED
			default:
				log.Panicf("Unknown state: %s\n", state)
			}
			err = s.SaveJob(job)
			if err != nil {
				log.Printf("Failed to save job while handling update: %s\n", err)
			}
			return nil
		}),
		scheduler.Event_SUBSCRIBED: eventrules.New(
			controller.TrackSubscription(fidStore, time.Second*10),
		),
		scheduler.Event_OFFERS: eventrules.HandleF(func(ctx context.Context, e *scheduler.Event) error {
			offers := e.GetOffers().GetOffers()
			log.Printf("Received offers: %d\n", len(offers))
			/*
			 * TODO possible to write more efficient offers handling.
			 * Now with offers (order matters):
			 * - O1{mem: 10}
			 * - 02{mem: 20}
			 * and jobs:
			 * - J1{mem: 20}
			 * - J2{mem: 10}
			 * none offer will be accepted.
			 */
			runnable, err := s.GetRunnableJobs()
			if err != nil {
				log.Printf("Failed to get runnable jobs: %s\n", err)
				return nil
			}
			for i := range offers {
				runnable = handleOffer(ctx, cli, &offers[i], runnable, vaultC, s)
			}
			return nil
		}),
	}.Otherwise(logger.HandleEvent))
}

func logAllEvents() eventrules.Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, ch eventrules.Chain) (context.Context, *scheduler.Event, error) {
		log.Printf("%+v\n", *e)
		return ch(ctx, e, err)
	}
}

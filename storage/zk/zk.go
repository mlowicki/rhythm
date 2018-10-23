package zk

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/mlowicki/rhythm/conf"
	zkcoord "github.com/mlowicki/rhythm/coordinator/zk"
	"github.com/mlowicki/rhythm/model"
	"github.com/mlowicki/rhythm/zkutil"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samuel/go-zookeeper/zk"
	log "github.com/sirupsen/logrus"
)

type state struct {
	FrameworkID string
}

const (
	jobsDir  = "jobs"
	stateDir = "state"
)

var tasksCleanupCount = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "storage_zookeeper_tasks_cleanups",
	Help: "Number of old tasks cleanups.",
})

func init() {
	prometheus.MustRegister(tasksCleanupCount)
}

func NewStorage(c *conf.StorageZK) (*storage, error) {
	s := &storage{
		dir:     "/" + c.Dir,
		addrs:   c.Addrs,
		timeout: c.Timeout,
		taskTTL: c.TaskTTL,
	}
	err := s.connect()
	if err != nil {
		return nil, err
	}
	acl, err := zkutil.AddAuth(s.conn, &c.Auth)
	if err != nil {
		return nil, err
	}
	s.acl = acl
	err = s.init()
	if err != nil {
		return nil, err
	}
	coordConf := &conf.CoordinatorZK{
		Dir:         c.Dir,
		Addrs:       c.Addrs,
		Timeout:     c.Timeout,
		Auth:        c.Auth,
		ElectionDir: "election/tasks_cleanup",
	}
	coord, err := zkcoord.New(coordConf)
	if err != nil {
		return nil, err
	}
	s.runTasksCleanupScheduler(coord)
	return s, nil
}

type storage struct {
	dir     string
	addrs   []string
	conn    *zk.Conn
	acl     func(perms int32) []zk.ACL
	timeout time.Duration
	taskTTL time.Duration
}

func (s *storage) runTasksCleanupScheduler(coord *zkcoord.Coordinator) {
	interval := time.Hour
	go func() {
		for {
			log.Info("Waiting until tasks cleanup leader")
			ctx := coord.WaitUntilLeader()
			log.Info("Elected as tasks cleanup leader")
			timer := time.After(interval)
		inner:
			for {
				select {
				case <-timer:
					log.Debug("Old tasks cleanup started")
					deleted, err := s.tasksCleanup(ctx)
					if err != nil {
						log.Errorf("Old tasks cleanup failed: %s", err)
					} else {
						log.Debugf("Old tasks cleanup finished. Deleted tasks: %d", deleted)
						tasksCleanupCount.Inc()
					}
					timer = time.After(interval)
				case <-ctx.Done():
					break inner
				}
			}
		}
	}()
}

func (s *storage) connect() error {
	conn, _, err := zk.Connect(s.addrs, s.timeout)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *storage) SetFrameworkID(id string) error {
	path := s.dir + "/" + stateDir
	payload, stat, err := s.conn.Get(path)
	version := stat.Version
	st := state{}
	err = json.Unmarshal(payload, &st)
	if err != nil {
		return err
	}
	st.FrameworkID = id
	est, err := json.Marshal(&st)
	if err != nil {
		return err
	}
	_, err = s.conn.Set(path, est, version)
	return err
}

func (s *storage) GetFrameworkID() (string, error) {
	st := state{}
	payload, _, err := s.conn.Get(s.dir + "/" + stateDir)
	err = json.Unmarshal(payload, &st)
	if err != nil {
		return "", err
	}
	return st.FrameworkID, nil
}

func (s *storage) init() error {
	exists, _, err := s.conn.Exists(s.dir)
	if err != nil {
		return err
	}
	if !exists {
		_, err = s.conn.Create(s.dir, []byte{}, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
	}
	path := s.dir + "/" + stateDir
	exists, _, err = s.conn.Exists(path)
	if err != nil {
		return err
	}
	if !exists {
		st := state{}
		est, err := json.Marshal(&st)
		if err != nil {
			return err
		}
		_, err = s.conn.Create(path, est, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}

	}
	path = s.dir + "/" + jobsDir
	exists, _, err = s.conn.Exists(path)
	if err != nil {
		return err
	}
	if !exists {
		_, err = s.conn.Create(path, []byte{}, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) GetJob(group string, project string, id string) (*model.Job, error) {
	jobs, err := s.GetJobs()
	if err != nil {
		return nil, err
	}
	for _, job := range jobs {
		if job.Group == group && job.Project == project && job.ID == id {
			return job, nil
		}
	}
	return nil, nil
}

func (s *storage) GetGroupJobs(group string) ([]*model.Job, error) {
	jobs, err := s.GetJobs()
	if err != nil {
		return []*model.Job{}, err
	}
	filtered := make([]*model.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Group == group {
			filtered = append(filtered, job)
		}
	}
	return filtered, nil
}

func (s *storage) GetProjectJobs(group string, project string) ([]*model.Job, error) {
	jobs, err := s.GetJobs()
	if err != nil {
		return []*model.Job{}, err
	}
	filtered := make([]*model.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Group == group && job.Project == project {
			filtered = append(filtered, job)
		}
	}
	return filtered, nil
}

func (s *storage) GetJobs() ([]*model.Job, error) {
	jobs := []*model.Job{}
	base := s.dir + "/" + jobsDir
	children, _, err := s.conn.Children(base)
	if err != nil {
		return jobs, err
	}
	for _, child := range children {
		payload, _, err := s.conn.Get(base + "/" + child)
		if err != nil {
			return jobs, err
		}
		var job model.Job
		err = json.Unmarshal(payload, &job)
		if err != nil {
			return jobs, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

func (s *storage) GetTasks(group, project, id string) ([]*model.Task, error) {
	tasks := []*model.Task{}
	base := s.dir + "/" + jobsDir + "/" + group + ":" + project + ":" + id
	children, _, err := s.conn.Children(base)
	if err != nil {
		if err == zk.ErrNoNode {
			return tasks, nil
		}
		return tasks, err
	}
	for _, child := range children {
		payload, _, err := s.conn.Get(base + "/" + child)
		if err != nil {
			return tasks, err
		}
		var task model.Task
		err = json.Unmarshal(payload, &task)
		if err != nil {
			return tasks, err
		}
		tasks = append(tasks, &task)
	}
	return tasks, nil
}

func (s *storage) GetRunnableJobs() ([]*model.Job, error) {
	runnable := []*model.Job{}
	jobs, err := s.GetJobs()
	if err != nil {
		return runnable, err
	}
	for _, job := range jobs {
		if job.IsRunnable() {
			runnable = append(runnable, job)
		}
	}
	rand.Shuffle(len(runnable), func(i, j int) {
		runnable[i], runnable[j] = runnable[j], runnable[i]
	})
	return runnable, nil
}

func (s *storage) tasksCleanup(ctx context.Context) (int64, error) {
	deleted := int64(0)
	base := s.dir + "/" + jobsDir
	jobIDs, _, err := s.conn.Children(base)
	if err != nil {
		return 0, err
	}
	if ctx.Err() != nil {
		return deleted, nil
	}
	for _, jobID := range jobIDs {
		keys, _, err := s.conn.Children(base + "/" + jobID)
		if err != nil {
			log.Errorf("Failed getting task IDs: %s", err)
			continue
		}
		if ctx.Err() != nil {
			return deleted, nil
		}
		for _, key := range keys {
			chunks := strings.SplitN(key, "@", 2)
			timestamp, err := strconv.ParseInt(chunks[0], 10, 64)
			if err != nil {
				log.Errorf("Failed parsing timestamp: %s", err)
				continue
			}
			if time.Now().Sub(time.Unix(timestamp, 0)) > s.taskTTL {
				err = s.conn.Delete(base+"/"+jobID+"/"+key, 0)
				if err != nil {
					log.Errorf("Failed removing old task: %s", err)
					continue
				}
				deleted += 1
			}
			if ctx.Err() != nil {
				return deleted, nil
			}
		}
	}
	return deleted, nil
}

func (s *storage) AddTask(group, project, id string, task *model.Task) error {
	encoded, err := json.Marshal(task)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/%s:%s:%s/%d@%s", s.dir+"/"+jobsDir, group, project,
		id, task.End.Unix(), task.TaskID)
	_, err = s.conn.Create(path, encoded, 0, s.acl(zk.PermAll))
	return err
}

func (s *storage) SaveJob(job *model.Job) error {
	encoded, err := json.Marshal(job)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/%s:%s:%s", s.dir+"/"+jobsDir, job.Group, job.Project, job.ID)
	exists, stat, err := s.conn.Exists(path)
	if err != nil {
		return err
	}
	if exists {
		_, err = s.conn.Set(path, encoded, stat.Version)
		if err != nil {
			return err
		}
	} else {
		_, err = s.conn.Create(path, encoded, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) DeleteJob(group string, project string, id string) error {
	jobPath := fmt.Sprintf("%s/%s:%s:%s", s.dir+"/"+jobsDir, group, project, id)
	exists, stat, err := s.conn.Exists(jobPath)
	if err != nil {
		return err
	}
	if exists {
		keys, _, err := s.conn.Children(jobPath)
		if err != nil {
			return err
		}
		for _, key := range keys {
			err = s.conn.Delete(jobPath+"/"+key, 0)
			if err != nil {
				return err
			}
		}
		err = s.conn.Delete(jobPath, stat.Version)
		if err != nil {
			return err
		}
	}
	return nil
}

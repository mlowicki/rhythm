package zk

import (
	"context"
	"encoding/json"
	"fmt"
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

type frameworkState struct {
	FrameworkID string
}

const (
	jobsDir           = "jobs"
	queuedJobsDir     = "queuedJobs"
	jobTasksDir       = "tasks"
	jobRuntimeDir     = "runtime"
	frameworkStateDir = "state"
)

var tasksCleanupCount = prometheus.NewCounter(prometheus.CounterOpts{
	Name: "storage_zookeeper_tasks_cleanups",
	Help: "Number of old tasks cleanups.",
})

func init() {
	prometheus.MustRegister(tasksCleanupCount)
}

// New creates fresh instance of ZooKeeper-backed storage.
func New(c *conf.StorageZK) (*storage, error) {
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
	path := s.dir + "/" + frameworkStateDir
	payload, stat, err := s.conn.Get(path)
	if err != nil {
		return err
	}
	version := stat.Version
	st := frameworkState{}
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
	st := frameworkState{}
	payload, _, err := s.conn.Get(s.dir + "/" + frameworkStateDir)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(payload, &st)
	if err != nil {
		return "", err
	}
	return st.FrameworkID, nil
}

func (s *storage) init() error {
	_, err := s.conn.Create(s.dir, []byte{}, 0, s.acl(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}
	state := frameworkState{}
	encodedState, err := json.Marshal(&state)
	if err != nil {
		return err
	}
	_, err = s.conn.Create(s.dir+"/"+frameworkStateDir, encodedState, 0, s.acl(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}
	_, err = s.conn.Create(s.dir+"/"+jobsDir, []byte{}, 0, s.acl(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}
	_, err = s.conn.Create(s.dir+"/"+queuedJobsDir, []byte{}, 0, s.acl(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}
	return nil
}

func (s *storage) GetJobRuntime(groupID, projectID, jobID string) (*model.JobRuntime, error) {
	fqid := groupID + ":" + projectID + ":" + jobID
	jobPath := s.dir + "/" + jobsDir + "/" + fqid + "/" + jobRuntimeDir
	encodedJob, _, err := s.conn.Get(jobPath)
	if err != nil {
		if err == zk.ErrNoNode {
			return nil, nil
		}
		return nil, err
	}
	var job model.JobRuntime
	err = json.Unmarshal(encodedJob, &job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *storage) GetJobConf(groupID, projectID, jobID string) (*model.JobConf, error) {
	fqid := groupID + ":" + projectID + ":" + jobID
	jobPath := s.dir + "/" + jobsDir + "/" + fqid
	encodedJob, _, err := s.conn.Get(jobPath)
	if err != nil {
		if err == zk.ErrNoNode {
			return nil, nil
		}
		return nil, err
	}
	var job model.JobConf
	err = json.Unmarshal(encodedJob, &job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (s *storage) GetJob(groupID, projectID, jobID string) (*model.Job, error) {
	conf, err := s.GetJobConf(groupID, projectID, jobID)
	if err != nil {
		return nil, err
	}
	if conf == nil {
		return nil, nil
	}
	runtime, err := s.GetJobRuntime(groupID, projectID, jobID)
	if err != nil {
		return nil, err
	}
	if runtime == nil {
		return nil, nil
	}
	job := model.Job{JobConf: *conf, JobRuntime: *runtime}
	return &job, nil
}

func (s *storage) GetGroupJobs(groupID string) ([]*model.Job, error) {
	jobs, err := s.GetJobs()
	if err != nil {
		return nil, err
	}
	filtered := make([]*model.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Group == groupID {
			filtered = append(filtered, job)
		}
	}
	return filtered, nil
}

func (s *storage) GetProjectJobs(groupID, projectID string) ([]*model.Job, error) {
	jobs, err := s.GetJobs()
	if err != nil {
		return nil, err
	}
	filtered := make([]*model.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Group == groupID && job.Project == projectID {
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
		encodedJobConf, _, err := s.conn.Get(base + "/" + child)
		if err != nil {
			return jobs, err
		}
		var job model.Job
		err = json.Unmarshal(encodedJobConf, &job)
		if err != nil {
			return jobs, err
		}
		encodedJobRuntime, _, err := s.conn.Get(base + "/" + child + "/" + jobRuntimeDir)
		if err != nil {
			return jobs, err
		}
		err = json.Unmarshal(encodedJobRuntime, &job)
		if err != nil {
			return jobs, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

func (s *storage) GetTasks(groupID, projectID, jobID string) ([]*model.Task, error) {
	tasks := []*model.Task{}
	fqid := groupID + ":" + projectID + ":" + jobID
	base := s.dir + "/" + jobsDir + "/" + fqid + "/" + jobTasksDir
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

func (s *storage) tasksCleanup(ctx context.Context) (int64, error) {
	deleted := int64(0)
	jobsPath := s.dir + "/" + jobsDir
	jobIDs, _, err := s.conn.Children(jobsPath)
	if err != nil {
		return 0, err
	}
	if ctx.Err() != nil {
		return deleted, nil
	}
	for _, jobID := range jobIDs {
		tasksPath := jobsPath + "/" + jobID + "/" + jobTasksDir
		keys, _, err := s.conn.Children(tasksPath)
		if err != nil {
			log.Errorf("Failed getting tasks IDs: %s", err)
			continue
		}
		if ctx.Err() != nil {
			return deleted, nil
		}
		for _, key := range keys {
			chunks := strings.SplitN(key, "@", 2)
			timestamp, err := strconv.ParseInt(chunks[0], 10, 64)
			if err != nil {
				log.Errorf("Failed parsing task timestamp: %s", err)
				continue
			}
			if time.Now().Sub(time.Unix(timestamp, 0)) > s.taskTTL {
				err = s.conn.Delete(tasksPath+"/"+key, 0)
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

func (s *storage) AddTask(groupID, projectID, jobID string, task *model.Task) error {
	encoded, err := json.Marshal(task)
	if err != nil {
		return err
	}
	fqid := groupID + ":" + projectID + ":" + jobID
	tasksPath := s.dir + "/" + jobsDir + "/" + fqid + "/" + jobTasksDir
	taskPath := fmt.Sprintf("%s/%d@%s", tasksPath, task.End.Unix(), task.TaskID)
	_, err = s.conn.Create(taskPath, encoded, 0, s.acl(zk.PermAll))
	if err != nil {
		if err != zk.ErrNoNode {
			return err
		}
		_, err = s.conn.Create(tasksPath, []byte{}, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
		_, err = s.conn.Create(taskPath, encoded, 0, s.acl(zk.PermAll))
	}
	return err
}

func (s *storage) SaveJobConf(job *model.JobConf) error {
	encoded, err := json.Marshal(job)
	if err != nil {
		return err
	}
	fqid := job.Group + ":" + job.Project + ":" + job.ID
	jobPath := s.dir + "/" + jobsDir + "/" + fqid
	_, err = s.conn.Set(jobPath, encoded, -1)
	if err != nil {
		if err != zk.ErrNoNode {
			return err
		}
		_, err = s.conn.Create(jobPath, encoded, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) SaveJobRuntime(groupID, projectID, jobID string, job *model.JobRuntime) error {
	encoded, err := json.Marshal(job)
	if err != nil {
		return err
	}
	fqid := groupID + ":" + projectID + ":" + jobID
	jobPath := s.dir + "/" + jobsDir + "/" + fqid + "/" + jobRuntimeDir
	_, err = s.conn.Set(jobPath, encoded, -1)
	if err != nil {
		if err != zk.ErrNoNode {
			return err
		}
		_, err = s.conn.Create(jobPath, encoded, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) SaveJob(job *model.Job) error {
	err := s.SaveJobConf(&job.JobConf)
	if err != nil {
		return err
	}
	err = s.SaveJobRuntime(job.Group, job.Project, job.ID, &job.JobRuntime)
	if err != nil {
		return err
	}
	return nil
}

func (s *storage) GetQueuedJobsIDs() ([]model.JobID, error) {
	children, _, err := s.conn.Children(s.dir + "/" + queuedJobsDir)
	var ids []model.JobID
	for _, child := range children {
		chunks := strings.Split(child, ":")
		id := model.JobID{Group: chunks[0], Project: chunks[1], ID: chunks[2]}
		ids = append(ids, id)
	}
	return ids, err
}

func (s *storage) DequeueJob(groupID, projectID, jobID string) error {
	fqid := groupID + ":" + projectID + ":" + jobID
	path := s.dir + "/" + queuedJobsDir + "/" + fqid
	err := s.conn.Delete(path, 0)
	if err != nil && err != zk.ErrNoNode {
		return err
	}
	return nil
}

func (s *storage) QueueJob(groupID, projectID, jobID string) error {
	fqid := groupID + ":" + projectID + ":" + jobID
	path := s.dir + "/" + queuedJobsDir + "/" + fqid
	_, err := s.conn.Create(path, []byte{}, 0, s.acl(zk.PermAll))
	if err != nil && err != zk.ErrNodeExists {
		return err
	}
	return nil
}

func (s *storage) DeleteJob(groupID, projectID, jobID string) error {
	fqid := groupID + ":" + projectID + ":" + jobID
	jobPath := s.dir + "/" + jobsDir + "/" + fqid
	// delete tasks
	tasksPath := jobPath + "/" + jobTasksDir
	tasks, _, err := s.conn.Children(tasksPath)
	if err != nil && err != zk.ErrNoNode {
		return err
	}
	for _, task := range tasks {
		err = s.conn.Delete(tasksPath+"/"+task, 0)
		if err != nil && err != zk.ErrNoNode {
			return err
		}
	}
	err = s.conn.Delete(tasksPath, -1)
	if err != nil && err != zk.ErrNoNode {
		return err
	}
	// delete runtime node
	err = s.conn.Delete(jobPath+"/"+jobRuntimeDir, -1)
	if err != nil && err != zk.ErrNoNode {
		return err
	}
	// delete root (one including conf) node
	err = s.conn.Delete(jobPath, -1)
	if err != nil && err != zk.ErrNoNode {
		return err
	}
	return nil
}

package zk

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	"github.com/mlowicki/rhythm/zkutil"
	"github.com/robfig/cron"
	"github.com/samuel/go-zookeeper/zk"
)

type rootDirConfig struct {
	FrameworkID string
}

func NewStorage(c *conf.StorageZK) (*storage, error) {
	s := &storage{
		rootDirPath: c.BasePath,
		jobsDirName: "jobs",
		servers:     c.Servers,
		timeout:     c.Timeout,
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
	return s, nil
}

type storage struct {
	rootDirPath string
	jobsDirName string
	servers     []string
	conn        *zk.Conn
	acl         func(perms int32) []zk.ACL
	timeout     time.Duration
}

func (s *storage) connect() error {
	conn, _, err := zk.Connect(s.servers, s.timeout)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *storage) SetFrameworkID(id string) error {
	payload, stat, err := s.conn.Get(s.rootDirPath)
	version := stat.Version
	conf := rootDirConfig{}
	err = json.Unmarshal(payload, &conf)
	if err != nil {
		return err
	}
	conf.FrameworkID = id
	encoded, err := json.Marshal(&conf)
	if err != nil {
		return err
	}
	_, err = s.conn.Set(s.rootDirPath, encoded, version)
	return nil
}

func (s *storage) GetFrameworkID() (string, error) {
	conf := rootDirConfig{}
	payload, _, err := s.conn.Get(s.rootDirPath)
	err = json.Unmarshal(payload, &conf)
	if err != nil {
		return "", err
	}
	if conf.FrameworkID == "" {
		return "", nil
	}
	return conf.FrameworkID, nil
}

func (s *storage) init() error {
	exists, _, err := s.conn.Exists(s.rootDirPath)
	if exists {
		return nil
	}
	conf := rootDirConfig{}
	encoded, err := json.Marshal(&conf)
	if err != nil {
		return err
	}
	_, err = s.conn.Create(s.rootDirPath, encoded, 0, s.acl(zk.PermAll))
	if err != nil {
		return err
	}
	jobsPath := s.rootDirPath + "/" + s.jobsDirName
	exists, _, err = s.conn.Exists(jobsPath)
	if err != nil {
		return err
	}
	if !exists {
		_, err = s.conn.Create(jobsPath, []byte{}, 0, s.acl(zk.PermAll))
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
	jobsPath := s.rootDirPath + "/" + s.jobsDirName
	children, _, err := s.conn.Children(jobsPath)
	if err != nil {
		return jobs, err
	}
	for _, child := range children {
		payload, _, err := s.conn.Get(jobsPath + "/" + child)
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

func (s *storage) GetRunnableJobs() ([]*model.Job, error) {
	runnable := []*model.Job{}
	jobs, err := s.GetJobs()
	if err != nil {
		return runnable, err
	}

	for _, job := range jobs {
		if job.State == model.RUNNING {
			continue
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if job.Schedule.Kind != model.Cron { // TODO
			panic("Only Cron schedules are supported")
		}
		sched, err := parser.Parse(job.Schedule.Cron)
		if err != nil {
			panic(err)
		}

		var t time.Time

		if job.LastStartAt.Before(job.CreatedAt) {
			t = job.CreatedAt
		} else {
			t = job.LastStartAt
		}

		if sched.Next(t).Before(time.Now()) {
			runnable = append(runnable, job)
		}
	}

	rand.Shuffle(len(runnable), func(i, j int) {
		runnable[i], runnable[j] = runnable[j], runnable[i]
	})

	return runnable, nil
}

func (s *storage) SaveJob(job *model.Job) error {
	encoded, err := json.Marshal(job)
	if err != nil {
		return err
	}
	jobsPath := s.rootDirPath + "/" + s.jobsDirName
	jobPath := fmt.Sprintf("%s/%s:%s:%s", jobsPath, job.Group, job.Project, job.ID)
	exists, stat, err := s.conn.Exists(jobPath)
	if err != nil {
		return err
	}
	if exists {
		_, err = s.conn.Set(jobPath, encoded, stat.Version)
		if err != nil {
			return err
		}
	} else {
		_, err = s.conn.Create(jobPath, encoded, 0, s.acl(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storage) DeleteJob(group string, project string, id string) error {
	path := fmt.Sprintf("%s/%s:%s:%s", s.rootDirPath+"/"+s.jobsDirName, group, project, id)
	exists, stat, err := s.conn.Exists(path)
	if err != nil {
		return err
	}
	if exists {
		err = s.conn.Delete(path, stat.Version)
		if err != nil {
			return err
		}
	}
	return nil
}

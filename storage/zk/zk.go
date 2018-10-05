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
	log "github.com/sirupsen/logrus"
)

type state struct {
	FrameworkID string
}

const (
	jobsDir  = "jobs"
	stateDir = "state"
)

func NewStorage(c *conf.StorageZK) (*storage, error) {
	s := &storage{
		dir:     "/" + c.Dir,
		servers: c.Servers,
		timeout: c.Timeout,
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
	dir     string
	servers []string
	conn    *zk.Conn
	acl     func(perms int32) []zk.ACL
	timeout time.Duration
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

func (s *storage) GetRunnableJobs() ([]*model.Job, error) {
	runnable := []*model.Job{}
	jobs, err := s.GetJobs()
	if err != nil {
		return runnable, err
	}

	for _, job := range jobs {
		if job.State != model.IDLE && job.State != model.FAILED {
			continue
		}
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if job.Schedule.Kind != model.Cron {
			log.Panic("Only Cron schedule is supported")
		}
		sched, err := parser.Parse(job.Schedule.Cron)
		if err != nil {
			log.WithFields(log.Fields{
				"cron": job.Schedule.Cron,
			}).Errorf("Cron schedule failed parsing: %s", err)
			continue
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
	path := fmt.Sprintf("%s/%s:%s:%s", s.dir+"/"+jobsDir, group, project, id)
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

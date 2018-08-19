package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	"github.com/robfig/cron"
	"github.com/samuel/go-zookeeper/zk"
)

type ZKRootDirConfig struct {
	FrameworkID string
}

func newZKStorage(c *conf.ZooKeeper) (*ZKStorage, error) {
	storage := &ZKStorage{
		rootDirPath: c.BasePath,
		jobsDirName: "jobs",
		servers:     c.Servers,
		timeout:     c.Timeout,
	}
	err := storage.connect()
	if err != nil {
		return nil, err
	}
	err = storage.init()
	if err != nil {
		return nil, err
	}
	return storage, nil
}

type ZKStorage struct {
	rootDirPath string
	jobsDirName string
	servers     []string
	conn        *zk.Conn
	timeout     time.Duration
}

func (s *ZKStorage) connect() error {
	conn, _, err := zk.Connect(s.servers, s.timeout)
	if err != nil {
		return err
	}
	s.conn = conn
	return nil
}

func (s *ZKStorage) SetFrameworkID(id string) error {
	payload, stat, err := s.conn.Get(s.rootDirPath)
	version := stat.Version
	conf := ZKRootDirConfig{}
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

func (s *ZKStorage) GetFrameworkID() (string, error) {
	conf := ZKRootDirConfig{}
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

func (s *ZKStorage) init() error {
	exists, _, err := s.conn.Exists(s.rootDirPath)
	if exists {
		return nil
	}
	conf := ZKRootDirConfig{}
	encoded, err := json.Marshal(&conf)
	if err != nil {
		return err
	}
	_, err = s.conn.Create(s.rootDirPath, encoded, 0, zk.WorldACL(zk.PermAll))
	if err != nil {
		return err
	}
	jobsPath := s.rootDirPath + "/" + s.jobsDirName
	exists, _, err = s.conn.Exists(jobsPath)
	if err != nil {
		return err
	}
	if !exists {
		_, err = s.conn.Create(jobsPath, []byte{}, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ZKStorage) GetJob(group string, project string, id string) (*model.Job, error) {
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

func (s *ZKStorage) GetGroupJobs(group string) ([]*model.Job, error) {
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

func (s *ZKStorage) GetProjectJobs(group string, project string) ([]*model.Job, error) {
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

func (s *ZKStorage) GetJobs() ([]*model.Job, error) {
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

func (s *ZKStorage) GetRunnableJobs() ([]*model.Job, error) {
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
			log.Printf("Found job eligible to run: %s\n", job.ID)
			//log.Printf("Found job eligible to run: %s (%s)\n", job.ID, job.project.ID)
			runnable = append(runnable, job)
		}
	}

	rand.Shuffle(len(runnable), func(i, j int) {
		runnable[i], runnable[j] = runnable[j], runnable[i]
	})

	return runnable, nil
}

func (s *ZKStorage) SaveJob(job *model.Job) error {
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
		_, err = s.conn.Create(jobPath, encoded, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *ZKStorage) DeleteJob(group string, project string, id string) error {
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

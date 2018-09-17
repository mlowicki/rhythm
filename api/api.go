package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mlowicki/rhythm/api/auth"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	log "github.com/sirupsen/logrus"
)

var (
	errForbidden        = errors.New("Forbidden")
	errUnauthorized     = errors.New("Unauthorized")
	errJobAlreadyExists = errors.New("Job already exists")
	errJobNotFound      = errors.New("Job not found")
)

type authorizer interface {
	GetProjectAccessLevel(r *http.Request, group string, project string) (auth.AccessLevel, error)
}

type storage interface {
	GetJobs() ([]*model.Job, error)
	GetGroupJobs(group string) ([]*model.Job, error)
	GetProjectJobs(group string, project string) ([]*model.Job, error)
	GetJob(group string, project string, id string) (*model.Job, error)
	SaveJob(j *model.Job) error
	DeleteJob(group string, project string, id string) error
}

type handler struct {
	a authorizer
	s storage
	h func(auth authorizer, s storage, w http.ResponseWriter, r *http.Request) error
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.h(h.a, h.s, w, r)
	if err != nil {
		json.NewEncoder(w).Encode(struct {
			Error string
		}{err.Error()})
	}
}

func filterReadableJobs(a authorizer, r *http.Request, jobs []*model.Job) ([]*model.Job, error) {
	readable := make([]*model.Job, 0, len(jobs))
	lvls := make(map[string]auth.AccessLevel)
	var err error
	for _, job := range jobs {
		key := fmt.Sprintf("%s/%s", job.Group, job.Project)
		lvl, found := lvls[key]
		if !found {
			lvl, err = a.GetProjectAccessLevel(r, job.Group, job.Project)
			if err != nil {
				return nil, err
			}
			lvls[key] = lvl
		}
		if lvl != auth.NoAccess {
			readable = append(readable, job)
		}
	}
	return readable, nil
}

func getJobs(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	jobs, err := s.GetJobs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	readable, err := filterReadableJobs(a, r, jobs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	json.NewEncoder(w).Encode(readable)
	return nil
}

func getGroupJobs(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	jobs, err := s.GetGroupJobs(mux.Vars(r)["group"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	readable, err := filterReadableJobs(a, r, jobs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	json.NewEncoder(w).Encode(readable)
	return nil
}

func getProjectJobs(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	group := vars["group"]
	project := vars["project"]
	lvl, err := a.GetProjectAccessLevel(r, group, project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if lvl == auth.NoAccess {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	jobs, err := s.GetProjectJobs(group, project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	json.NewEncoder(w).Encode(jobs)
	return nil
}

func getJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	group := vars["group"]
	project := vars["project"]
	lvl, err := a.GetProjectAccessLevel(r, group, project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if lvl == auth.NoAccess {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	job, err := s.GetJob(group, project, vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(w).Encode(job)
	}
	return nil
}

func deleteJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	group := vars["group"]
	project := vars["project"]
	lvl, err := a.GetProjectAccessLevel(r, group, project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if lvl != auth.ReadWrite {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	err = s.DeleteJob(group, project, vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

type newJobPayload struct {
	Group    string
	Project  string
	ID       string
	Schedule struct {
		Cron string
	}
	Env       map[string]string
	Secrets   map[string]string
	Container struct {
		Docker *struct {
			Image          string
			ForcePullImage bool
		}
		Mesos *struct {
			Image string
		}
	}
	CPUs      float64
	Mem       float64
	Cmd       string
	User      string
	Shell     *bool
	Arguments []string
	Labels    map[string]string
}

func createJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	var payload newJobPayload
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("JSON decoding failed: %s", err)
	}
	lvl, err := a.GetProjectAccessLevel(r, payload.Group, payload.Project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if lvl != auth.ReadWrite {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	j := &model.Job{
		Group:   payload.Group,
		Project: payload.Project,
		ID:      payload.ID,
		Schedule: model.JobSchedule{
			Kind: model.Cron,
			Cron: payload.Schedule.Cron,
		},
		CreatedAt: time.Now(),
		Env:       payload.Env,
		Secrets:   payload.Secrets,
		Container: model.JobContainer{},
		CPUs:      payload.CPUs,
		Mem:       payload.Mem,
		Cmd:       payload.Cmd,
		User:      payload.User,
		Arguments: payload.Arguments,
		Labels:    payload.Labels,
	}
	if payload.Container.Docker != nil {
		j.Container.Kind = model.Docker
		j.Container.Docker = model.JobDocker{
			Image:          payload.Container.Docker.Image,
			ForcePullImage: payload.Container.Docker.ForcePullImage,
		}
	} else if payload.Container.Mesos != nil {
		j.Container.Kind = model.Mesos
		j.Container.Mesos = model.JobMesos{
			Image: payload.Container.Mesos.Image,
		}
	} else {
		return fmt.Errorf("Set container type (Mesos or Docker)")
	}
	if payload.Shell == nil {
		j.Shell = true
	} else {
		j.Shell = *payload.Shell
	}
	job, err := s.GetJob(payload.Group, payload.Project, payload.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if job != nil {
		w.WriteHeader(http.StatusBadRequest)
		return errJobAlreadyExists
	}
	err = s.SaveJob(j)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

type updateJobPayload struct {
	Schedule *struct {
		Cron *string
	}
	Env       *map[string]string
	Secrets   *map[string]string
	Container *struct {
		Docker *struct {
			Image          *string
			ForcePullImage *bool
		}
		Mesos *struct {
			Image *string
		}
	}
	CPUs      *float64
	Mem       *float64
	Cmd       *string
	User      *string
	Shell     *bool
	Arguments *[]string
	Labels    *map[string]string
}

func updateJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	var payload updateJobPayload
	group := vars["group"]
	project := vars["project"]
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("JSON decoding failed: %s", err)
	}
	lvl, err := a.GetProjectAccessLevel(r, group, project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if lvl != auth.ReadWrite {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	job, err := s.GetJob(group, project, vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
		return errJobNotFound
	}
	if payload.Schedule != nil {
		schedule := job.Schedule
		if payload.Schedule.Cron != nil {
			schedule.Kind = model.Cron
			schedule.Cron = *payload.Schedule.Cron
		}
		job.Schedule = schedule
	}
	if payload.Env != nil {
		job.Env = *payload.Env
	}
	if payload.Secrets != nil {
		job.Secrets = *payload.Secrets
	}
	if payload.Container != nil {
		container := job.Container
		if payload.Container.Docker != nil {
			container.Kind = model.Docker
			if payload.Container.Docker.Image != nil {
				container.Docker.Image = *payload.Container.Docker.Image
			}
			if payload.Container.Docker.ForcePullImage != nil {
				container.Docker.ForcePullImage = *payload.Container.Docker.ForcePullImage
			}
		} else if payload.Container.Mesos != nil {
			container.Kind = model.Mesos
			if payload.Container.Mesos.Image != nil {
				container.Mesos.Image = *payload.Container.Mesos.Image
			}
		}
		job.Container = container
	}
	if payload.CPUs != nil {
		job.CPUs = *payload.CPUs
	}
	if payload.Mem != nil {
		job.Mem = *payload.Mem
	}
	if payload.Cmd != nil {
		job.Cmd = *payload.Cmd
	}
	if payload.User != nil {
		job.User = *payload.User
	}
	if payload.Shell != nil {
		job.Shell = *payload.Shell
	}
	if payload.Arguments != nil {
		job.Arguments = *payload.Arguments
	}
	if payload.Labels != nil {
		job.Labels = *payload.Labels
	}
	err = s.SaveJob(job)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

func New(c *conf.API, s storage) {
	r := mux.NewRouter()
	v1 := r.PathPrefix("/api/v1").Subrouter()
	var a authorizer
	switch c.Auth.Backend {
	case conf.APIAuthBackendGitLab:
		a = &auth.GitLabAuthorizer{BaseURL: c.Auth.GitLab.BaseURL}
	case conf.APIAuthBackendNone:
		a = &auth.NoneAuthorizer{}
	default:
		log.Fatalf("Unknown authorization backend: %s", c.Auth.Backend)
	}
	log.Printf("Authorization backend: %s", c.Auth.Backend)
	v1.Handle("/jobs", &handler{a, s, getJobs}).Methods("GET")
	v1.Handle("/jobs", &handler{a, s, createJob}).Methods("POST")
	v1.Handle("/jobs/{group}", &handler{a, s, getGroupJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}", &handler{a, s, getProjectJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{a, s, getJob}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{a, s, deleteJob}).Methods("DELETE")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{a, s, updateJob}).Methods("PUT")
	srv := &http.Server{
		Handler:      r,
		Addr:         c.Address,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		if c.CertFile != "" || c.KeyFile != "" {
			log.Fatal(srv.ListenAndServeTLS(c.CertFile, c.KeyFile))
		} else {
			log.Fatal(srv.ListenAndServe())
		}
	}()
}

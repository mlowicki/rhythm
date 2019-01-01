package api

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/hashicorp/go-multierror"
	"github.com/mlowicki/rhythm/api/auth"
	"github.com/mlowicki/rhythm/api/auth/gitlab"
	"github.com/mlowicki/rhythm/api/auth/ldap"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/xeipuuv/gojsonschema"
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

func encoder(w http.ResponseWriter) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "    ")
	w.Header().Set("Content-Type", "application/json")
	return enc
}

type storage interface {
	GetJobs() ([]*model.Job, error)
	GetGroupJobs(group string) ([]*model.Job, error)
	GetProjectJobs(group, project string) ([]*model.Job, error)
	GetJob(group, project, id string) (*model.Job, error)
	SaveJob(j *model.Job) error
	DeleteJob(group, project, id string) error
	GetTasks(group, project, id string) ([]*model.Task, error)
	GetJobConf(group, project, id string) (*model.JobConf, error)
	SaveJobConf(state *model.JobConf) error
	QueueJob(group, project, id string) error
}

type handler struct {
	a authorizer
	s storage
	h func(auth authorizer, s storage, w http.ResponseWriter, r *http.Request) error
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.h(h.a, h.s, w, r)
	if err != nil {
		log.Errorf("API handler error: %s", err)
		errs := make([]string, 0, 1)
		if merr, ok := err.(*multierror.Error); ok {
			for _, err := range merr.Errors {
				errs = append(errs, err.Error())
			}
		} else {
			errs = append(errs, err.Error())
		}
		encoder(w).Encode(struct{ Errors []string }{errs})
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
	encoder(w).Encode(readable)
	return nil
}

func getTasks(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
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
	tasks, err := s.GetTasks(group, project, vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].End.Before(tasks[j].End) })
	encoder(w).Encode(tasks)
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
	encoder(w).Encode(readable)
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
	encoder(w).Encode(jobs)
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
		encoder(w).Encode(job)
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
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func runJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
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
	err = s.QueueJob(group, project, vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
func validateSchema(payload gojsonschema.JSONLoader, schema gojsonschema.JSONLoader) error {
	res, err := gojsonschema.Validate(schema, payload)
	if err != nil {
		return err
	}
	if !res.Valid() {
		var errs *multierror.Error
		for _, err := range res.Errors() {
			errs = multierror.Append(errs, errors.New(err.String()))
		}
		return errs
	}
	return nil
}

func createJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	var payload newJobPayload
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("JSON decoding failed: %s", err)
	}
	schemaLoader := gojsonschema.NewGoLoader(newJobSchema)
	payloadLoader := gojsonschema.NewGoLoader(payload)
	err = validateSchema(payloadLoader, schemaLoader)
	if err != nil {
		return err
	}
	if payload.Env == nil {
		payload.Env = make(map[string]string)
	}
	if payload.Secrets == nil {
		payload.Secrets = make(map[string]string)
	}
	if payload.Arguments == nil {
		payload.Arguments = make([]string, 0)
	}
	if payload.Labels == nil {
		payload.Labels = make(map[string]string)
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
	jobConf := &model.JobConf{
		JobID: model.JobID{
			Group:   payload.Group,
			Project: payload.Project,
			ID:      payload.ID,
		},
		Schedule: model.JobSchedule{
			Type: model.Cron,
			Cron: payload.Schedule.Cron,
		},
		Env:        payload.Env,
		Secrets:    payload.Secrets,
		Container:  model.JobContainer{},
		CPUs:       payload.CPUs,
		Mem:        payload.Mem,
		Disk:       payload.Disk,
		Cmd:        payload.Cmd,
		User:       payload.User,
		Arguments:  payload.Arguments,
		Labels:     payload.Labels,
		MaxRetries: payload.MaxRetries,
	}
	jobRuntime := &model.JobRuntime{}
	job := &model.Job{JobConf: *jobConf, JobRuntime: *jobRuntime}
	if payload.Container.Docker.Image != "" {
		job.Container.Type = model.Docker
		job.Container.Docker = &model.JobDocker{
			Image:          payload.Container.Docker.Image,
			ForcePullImage: payload.Container.Docker.ForcePullImage,
		}
	} else {
		job.Container.Type = model.Mesos
		job.Container.Mesos = &model.JobMesos{
			Image: payload.Container.Mesos.Image,
		}
	}
	if payload.Shell == nil {
		job.Shell = true
	} else {
		job.Shell = *payload.Shell
	}
	storedJob, err := s.GetJob(payload.Group, payload.Project, payload.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if storedJob != nil {
		w.WriteHeader(http.StatusBadRequest)
		return errJobAlreadyExists
	}
	job.State = model.IDLE
	err = s.SaveJob(job)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
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
	schemaLoader := gojsonschema.NewGoLoader(updateJobSchema)
	payloadLoader := gojsonschema.NewGoLoader(payload)
	err = validateSchema(payloadLoader, schemaLoader)
	if err != nil {
		return err
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
	job, err := s.GetJobConf(group, project, vars["id"])
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
			schedule.Type = model.Cron
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
			container.Type = model.Docker
			container.Mesos = nil
			if container.Docker == nil {
				container.Docker = &model.JobDocker{}
			}
			if payload.Container.Docker.Image != nil {
				container.Docker.Image = *payload.Container.Docker.Image
			}
			if payload.Container.Docker.ForcePullImage != nil {
				container.Docker.ForcePullImage = *payload.Container.Docker.ForcePullImage
			}
			if container.Docker.Image == "" {
				w.WriteHeader(http.StatusBadRequest)
				return errors.New("container.docker.image is required")
			}
		} else if payload.Container.Mesos != nil {
			container.Type = model.Mesos
			container.Docker = nil
			if container.Mesos == nil {
				container.Mesos = &model.JobMesos{}
			}
			container.Mesos.Image = *payload.Container.Mesos.Image
		}
		job.Container = container
	}
	if payload.CPUs != nil {
		job.CPUs = *payload.CPUs
	}
	if payload.Mem != nil {
		job.Mem = *payload.Mem
	}
	if payload.Disk != nil {
		job.Disk = *payload.Disk
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
	if payload.MaxRetries != nil {
		job.MaxRetries = *payload.MaxRetries
	}
	err = s.SaveJobConf(job)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// State describes server info.
type State struct {
	IsLeader func() bool
	Version  string
}

// New creates instance of API server and runs it in separate goroutine.
func New(c *conf.API, s storage, state State) {
	r := mux.NewRouter()
	v1 := r.PathPrefix("/api/v1").Subrouter().StrictSlash(true)
	var (
		a   authorizer
		err error
	)
	switch c.Auth.Backend {
	case conf.APIAuthBackendGitLab:
		a, err = gitlab.New(&c.Auth.GitLab)
	case conf.APIAuthBackendNone:
		a = &auth.NoneAuthorizer{}
	case conf.APIAuthBackendLDAP:
		ldap.SetTimeout(c.Auth.LDAP.Timeout)
		a, err = ldap.New(&c.Auth.LDAP)
	default:
		log.Fatalf("Unknown authorization backend: %s", c.Auth.Backend)
	}
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Authorization backend: %s", c.Auth.Backend)
	v1.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoder(w).Encode(struct {
			ServerTime string
			Version    string
			Leader     bool
		}{
			time.Now().Format(time.UnixDate),
			state.Version,
			state.IsLeader(),
		})
	}))
	v1.Handle("/jobs", &handler{a, s, getJobs}).Methods("GET")
	v1.Handle("/jobs", &handler{a, s, createJob}).Methods("POST")
	v1.Handle("/jobs/{group}", &handler{a, s, getGroupJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}", &handler{a, s, getProjectJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{a, s, getJob}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{a, s, deleteJob}).Methods("DELETE")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{a, s, updateJob}).Methods("PUT")
	v1.Handle("/jobs/{group}/{project}/{id}/tasks", &handler{a, s, getTasks}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}/run", &handler{a, s, runJob}).Methods("POST")
	v1.Handle("/metrics", promhttp.Handler())
	tlsConf := &tls.Config{
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}
	srv := &http.Server{
		Handler:      r,
		Addr:         c.Addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConf,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
	}
	go func() {
		if c.CertFile != "" || c.KeyFile != "" {
			log.Fatal(srv.ListenAndServeTLS(c.CertFile, c.KeyFile))
		} else {
			log.Fatal(srv.ListenAndServe())
		}
	}()
}

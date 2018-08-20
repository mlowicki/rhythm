package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/mlowicki/rhythm/auth"
	"github.com/mlowicki/rhythm/conf"
	"github.com/mlowicki/rhythm/model"
)

var (
	errForbidden    = errors.New("Forbidden")
	errUnauthorized = errors.New("Unauthorized")
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
		json.NewEncoder(w).Encode(errResp{err.Error()})
	}
}

type errResp struct {
	Error string `json:"error"`
}

func filterReadableJobs(a authorizer, r *http.Request, jobs []*model.Job) ([]*model.Job, error) {
	readable := make([]*model.Job, 0, len(jobs))
	acl := make(map[string]auth.AccessLevel)
	var err error
	for _, job := range jobs {
		key := fmt.Sprintf("%s/%s", job.Group, job.Project)
		lvl, found := acl[key]
		if !found {
			lvl, err = a.GetProjectAccessLevel(r, job.Group, job.Project)
			if err != nil {
				return nil, err
			}
			acl[key] = lvl
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

func createJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		Group    string
		Project  string
		ID       string
		Schedule struct {
			Cron string
		}
		Env       map[string]string
		Container struct {
			Docker struct {
				Image string
			}
		}
		CPUs float64
		Mem  float64
		Cmd  string
		User string
	}
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
	// TODO input validation
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
		Container: model.JobContainer{
			Kind: model.Docker,
			Docker: model.JobDocker{
				Image: payload.Container.Docker.Image,
			},
		},
		CPUs: payload.CPUs,
		Mem:  payload.Mem,
		Cmd:  payload.Cmd,
		User: payload.User,
	}
	err = s.SaveJob(j)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

func updateJob(_ authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	log.Printf("Vars: %v\n", vars)
	// TODO
	return nil
}

func NewAPI(conf *conf.Conf, s storage) {
	r := mux.NewRouter()
	v1 := r.PathPrefix("/v1").Subrouter()
	auth := auth.GitLabAuthorizer{BaseURL: conf.GitLab.BaseURL}
	v1.Handle("/jobs", &handler{&auth, s, getJobs}).Methods("GET")
	v1.Handle("/jobs", &handler{&auth, s, createJob}).Methods("POST")
	v1.Handle("/jobs/{group}", &handler{&auth, s, getGroupJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}", &handler{&auth, s, getProjectJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{&auth, s, getJob}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{&auth, s, deleteJob}).Methods("DELETE")
	v1.Handle("/jobs/{group}/{project}/{id}", &handler{&auth, s, updateJob}).Methods("PUT")
	srv := &http.Server{
		Handler: r,
		Addr:    conf.API.Address,
		// TODO Enforce timeouts
		// TODO Support for HTTPS
	}
	go func() {
		log.Fatal(srv.ListenAndServe())
	}()
}

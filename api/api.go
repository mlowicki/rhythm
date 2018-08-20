package api

import (
	"encoding/json"
	"errors"
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

func getJobs(_ authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	jobs, err := s.GetJobs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	json.NewEncoder(w).Encode(jobs)
	return nil
}

func getGroupJobs(_ authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	jobs, err := s.GetGroupJobs(mux.Vars(r)["group"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	json.NewEncoder(w).Encode(jobs)
	return nil
}

func getProjectJobs(_ authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	jobs, err := s.GetProjectJobs(vars["group"], vars["project"])
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
	accessLevel, err := a.GetProjectAccessLevel(r, group, project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if accessLevel == auth.NoAccess {
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
	accessLevel, err := a.GetProjectAccessLevel(r, vars["group"], vars["project"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if accessLevel != auth.ReadWrite {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	err = s.DeleteJob(vars["group"], vars["project"], vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

func createJob(a authorizer, s storage, w http.ResponseWriter, r *http.Request) error {
	var payload struct {
		ID      string
		Project string
		Group   string
	}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&payload)
	if err != nil {
		// TODO
		log.Fatal(err)
	}
	accessLevel, err := a.GetProjectAccessLevel(r, payload.Group, payload.Project)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if accessLevel != auth.ReadWrite {
		w.WriteHeader(http.StatusForbidden)
		return errForbidden
	}
	// TODO input validation
	// TODO remove hardcoded values
	j := &model.Job{
		Group:   payload.Group,
		Project: payload.Project,
		ID:      payload.ID,
		Schedule: model.JobSchedule{
			Kind: model.Cron,
			Cron: "*/1 * * * *",
		},
		CreatedAt: time.Now(),
		Env: map[string]string{
			"BAR": "bar",
		},
		Cmd: "echo $BAR",
		Container: model.JobContainer{
			Kind: model.Docker,
			Docker: model.JobDocker{
				Image: "alpine:3.7",
			},
		},
		CPUs: 4,
		Mem:  7168,
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

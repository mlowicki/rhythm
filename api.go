package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type AccessLevel int

const (
	NoAccess AccessLevel = iota
	ReadOnly
	ReadWrite
)

var (
	apiErrAccessForbidden = fmt.Errorf("Access forbidden")
	apiErrUnauthorized    = fmt.Errorf("Unauthorized")
)

type Authorizer interface {
	GetProjectAccessLevel(r *http.Request, group string, project string) (AccessLevel, error)
}

type apiHandler struct {
	A Authorizer
	S Storage
	H func(auth Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error
}

func (h *apiHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := h.H(h.A, h.S, w, r)
	if err != nil {
		switch e := err.(type) {
		case *apiErr:
			w.WriteHeader(e.Status)
			err = json.NewEncoder(w).Encode(e)
			if err != nil {
				log.Fatal(err)
			}
		default:
			log.Fatal(err)
		}
	}
}

type apiErr struct {
	Message string `json:"error"`
	Status  int    `json:"-"`
}

func (e *apiErr) Error() string {
	return e.Message
}

func getJobs(_ Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
	jobs, err := s.GetJobs()
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	json.NewEncoder(w).Encode(jobs)
	return nil
}

func getGroupJobs(_ Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
	jobs, err := s.GetGroupJobs(mux.Vars(r)["group"])
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	json.NewEncoder(w).Encode(jobs)
	return nil
}

func getProjectJobs(_ Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	jobs, err := s.GetProjectJobs(vars["group"], vars["project"])
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	json.NewEncoder(w).Encode(jobs)
	return nil
}

func getJob(_ Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	job, err := s.GetJob(vars["group"], vars["project"], vars["id"])
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	if job == nil {
		w.WriteHeader(http.StatusNotFound)
	} else {
		json.NewEncoder(w).Encode(job)
	}
	return nil
}

func deleteJob(auth Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	accessLevel, err := auth.GetProjectAccessLevel(r, vars["group"], vars["project"])
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	if accessLevel != ReadWrite {
		return &apiErr{apiErrAccessForbidden.Error(), http.StatusForbidden}
	}
	err = s.DeleteJob(vars["group"], vars["project"], vars["id"])
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	return nil
}

func createJob(auth Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
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
	accessLevel, err := auth.GetProjectAccessLevel(r, payload.Group, payload.Project)
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	if accessLevel != ReadWrite {
		return &apiErr{apiErrAccessForbidden.Error(), http.StatusForbidden}
	}
	// TODO input validation
	// TODO remove hardcoded values
	j := &Job{
		Group:   payload.Group,
		Project: payload.Project,
		ID:      payload.ID,
		Schedule: JobSchedule{
			Kind: Cron,
			Cron: "*/1 * * * *",
		},
		CreatedAt: time.Now(),
		Env: map[string]string{
			"BAR": "bar",
		},
		Cmd: "echo $BAR",
		Container: JobContainer{
			Kind: Docker,
			Docker: JobDocker{
				Image: "alpine:3.7",
			},
		},
		CPUs: 4,
		Mem:  7168,
	}
	err = s.SaveJob(j)
	if err != nil {
		return &apiErr{err.Error(), http.StatusInternalServerError}
	}
	return nil
}

func updateJob(_ Authorizer, s Storage, w http.ResponseWriter, r *http.Request) error {
	vars := mux.Vars(r)
	log.Printf("Vars: %v\n", vars)
	// TODO
	return nil
}

func initAPI(conf *Config, s Storage) {
	r := mux.NewRouter()
	v1 := r.PathPrefix("/v1").Subrouter()
	auth := GitLabAuthorizer{Config: &conf.GitLab}
	v1.Handle("/jobs", &apiHandler{&auth, s, getJobs}).Methods("GET")
	v1.Handle("/jobs", &apiHandler{&auth, s, createJob}).Methods("POST")
	v1.Handle("/jobs/{group}", &apiHandler{&auth, s, getGroupJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}", &apiHandler{&auth, s, getProjectJobs}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &apiHandler{&auth, s, getJob}).Methods("GET")
	v1.Handle("/jobs/{group}/{project}/{id}", &apiHandler{&auth, s, deleteJob}).Methods("DELETE")
	v1.Handle("/jobs/{group}/{project}/{id}", &apiHandler{&auth, s, updateJob}).Methods("PUT")
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

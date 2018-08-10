package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

func initAPI(conf *Config, s Storage) {
	go func() {
		http.HandleFunc("/v1/task", func(w http.ResponseWriter, r *http.Request) {
			taskHandler(w, r, conf, s)
		})
		// TODO Support for HTTPS
		log.Fatal(http.ListenAndServe(conf.API.Address, nil))
	}()
}

// TODO decide based on method if task should be created or modified (POST vs PUT)
func taskHandler(w http.ResponseWriter, r *http.Request, conf *Config, s Storage) {
	client, err := newGitLabClient(conf, r.Header.Get("X-Token"))
	if err != nil {
		log.Fatal(err)
	}
	var payload struct {
		ID      string
		Project string
		Group   string
	}
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&payload)
	if err != nil {
		// TODO handle error gracefully + return error response
		log.Fatal(err)
	}
	path := fmt.Sprintf("%s/%s", payload.Group, payload.Project)
	project, _, err := client.Projects.GetProject(path)
	if err != nil {
		log.Printf("Error getting project's data: %s\n", err)
		// TODO return error response
		return
	}
	if getProjectAccessLevel(project) != ReadWrite {
		log.Printf("Forbidden to modify project")
		// TODO return error response
		return
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
		log.Printf("Failed to save job: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		// TODO return error response
		return
	}
	// TODO return error response
}

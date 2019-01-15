package command

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	prompt "github.com/c-bata/go-prompt"
	"github.com/mesos/mesos-go/api/v1/lib/backoff"
	"github.com/mlowicki/rhythm/command/apiclient"
	"github.com/mlowicki/rhythm/model"
)

// ClientCommand implements interactive client.
type ClientCommand struct {
	*BaseCommand
	addr                 string
	auth                 string
	Version              string
	jobs                 []*model.Job
	jobsMut              sync.Mutex
	group                string
	project              string
	serverCallsThrottler <-chan struct{}
	token                string
	username             string
	password             string
	apiClient            *apiclient.Client
}

var topLevelSuggestions = []prompt.Suggest{
	{"cd", "Go to group, project or back"},
	{"delete", "Delete job"},
	{"health", "Show server info"},
	{"ls", "List jobs"},
	{"read", "Show job configuration and state"},
	{"run", "Schedule job for immediate run"},
	{"tasks", "Show job tasks (runs)"},
}

func (c *ClientCommand) completeRead(word string, words []string) []prompt.Suggest {
	if (len(words) == 2 && word == "") || len(words) > 2 {
		return []prompt.Suggest{}
	}
	names := make(map[string]struct{})
	for _, job := range c.getJobs() {
		if c.group != "" && c.group != job.Group {
			continue
		}
		if c.project != "" && c.project != job.Project {
			continue
		}
		names[c.relativeJobID(job)] = struct{}{}
	}
	suggestions := make([]prompt.Suggest, 0, len(names))
	for key := range names {
		if len(words) == 1 || (len(words) > 1 && strings.Contains(key, words[1])) {
			suggestions = append(suggestions, prompt.Suggest{key, ""})
		}
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		return suggestions[i].Text < suggestions[j].Text
	})
	return suggestions
}

func (c *ClientCommand) getJobs() []*model.Job {
	go func() {
		select {
		case <-c.serverCallsThrottler:
			jobs, err := c.apiClient.FindJobs("")
			if err != nil {
				return
			}
			c.jobsMut.Lock()
			c.jobs = jobs
			c.jobsMut.Unlock()
		default:
		}
	}()
	c.jobsMut.Lock()
	jobs := c.jobs
	c.jobsMut.Unlock()
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Path() < jobs[j].Path()
	})
	return jobs
}

func (c *ClientCommand) completeCd(word string, words []string) []prompt.Suggest {
	if (len(words) == 2 && word == "") || len(words) > 2 {
		return []prompt.Suggest{}
	}
	names := make(map[string]struct{})
	if c.project == "" {
		for _, job := range c.getJobs() {
			if c.group != "" && c.group != job.Group {
				continue
			}
			if c.group != "" {
				names[job.Project] = struct{}{}
			} else {
				names[job.Group] = struct{}{}
			}
		}
	}
	if c.group != "" {
		names[".."] = struct{}{}
	}
	suggestions := make([]prompt.Suggest, 0, len(names))
	for key := range names {
		if len(words) == 1 || (len(words) > 1 && strings.HasPrefix(key, words[1])) {
			suggestions = append(suggestions, prompt.Suggest{key, ""})
		}
	}
	sort.SliceStable(suggestions, func(i, j int) bool {
		return suggestions[i].Text < suggestions[j].Text
	})
	return suggestions
}

func (c *ClientCommand) completeRun(word string, words []string) []prompt.Suggest {
	return c.completeRead(word, words)
}

func (c *ClientCommand) completeTasks(word string, words []string) []prompt.Suggest {
	return c.completeRead(word, words)
}

func (c *ClientCommand) completeDelete(word string, words []string) []prompt.Suggest {
	return c.completeRead(word, words)
}

func (c *ClientCommand) completer(in prompt.Document) []prompt.Suggest {
	words := strings.Fields(in.TextBeforeCursor())
	if len(words) == 0 {
		return topLevelSuggestions
	}
	word := in.GetWordBeforeCursor()
	if len(words) == 1 && word != "" {
		return prompt.FilterHasPrefix(topLevelSuggestions, word, true)
	}
	switch words[0] {
	case "cd":
		return c.completeCd(word, words)
	case "read":
		return c.completeRead(word, words)
	case "run":
		return c.completeRun(word, words)
	case "tasks":
		return c.completeTasks(word, words)
	case "delete":
		return c.completeDelete(word, words)
	}
	return []prompt.Suggest{}
}

func (c *ClientCommand) relativeJobID(job *model.Job) string {
	if c.project != "" {
		return job.ID
	} else if c.group != "" {
		return job.Project + "/" + job.ID
	}
	return job.Path()
}

func (c *ClientCommand) absoluteJobID(id string) string {
	if c.project != "" {
		id = c.project + "/" + id
	}
	if c.group != "" {
		id = c.group + "/" + id
	}
	return id
}

func (c *ClientCommand) listJobs(filter string) {
	jobs, err := c.apiClient.FindJobs(filter)
	if err != nil {
		c.Errorf("Error getting jobs: %s", err)
		return
	}
	for _, job := range jobs {
		if c.project != "" && c.project != job.Project {
			continue
		}
		if c.group != "" && c.group != job.Group {
			continue
		}
		fmt.Printf("%s %s\n", c.relativeJobID(job), coloredState(job.State))
	}
}

func (c *ClientCommand) changeLevel(in string) {
	if in == ".." {
		if c.project != "" {
			c.project = ""
		} else if c.group != "" {
			c.group = ""
		} else {
			c.Errorf("Outermost level reached.")
		}
		return
	}
	if c.group == "" {
		c.group = in
	} else if c.project == "" {
		c.project = in
	} else {
		c.Errorf("Innermost level reached.")
	}
}

func (c *ClientCommand) readJob(id string) {
	job, err := c.apiClient.ReadJob(id)
	if err != nil {
		c.Errorf("%s", err)
		return
	}
	c.printJob(job)
}

func (c *ClientCommand) deleteJob(id string) {
	err := c.apiClient.DeleteJob(id)
	if err != nil {
		c.Errorf("%s", err)
		return
	}
	c.getJobs()
}

func (c *ClientCommand) health() {
	health, err := c.apiClient.Health()
	if err != nil {
		c.Errorf("%s", err)
		return
	}
	c.printHealth(health)
}

func (c *ClientCommand) run(id string) {
	err := c.apiClient.RunJob(id)
	if err != nil {
		c.Errorf("%s", err)
		return
	}
	c.Printf("Job scheduled for immedidate run.")
}

func (c *ClientCommand) readTasks(id string) {
	tasks, err := c.apiClient.ReadTasks(id)
	if err != nil {
		c.Errorf("%s", err)
		return
	}
	c.printTasks(tasks)
}

func (c *ClientCommand) executor(in string) {
	blocks := strings.Fields(in)
	if len(blocks) == 0 {
		return
	}
	switch blocks[0] {
	case "exit":
		c.Printf("Bye!")
		os.Exit(0)
		return
	case "ls":
		filter := ""
		if len(blocks) > 1 {
			filter = blocks[1]
		}
		c.listJobs(filter)
	case "cd":
		if len(blocks) == 1 {
			c.Errorf("Argument is missing.")
			return
		}
		c.changeLevel(blocks[1])
	case "read":
		if len(blocks) == 1 {
			c.Errorf("Argument is missing.")
			return
		}
		c.readJob(c.absoluteJobID(blocks[1]))
	case "health":
		c.health()
	case "run":
		if len(blocks) == 1 {
			c.Errorf("Argument is missing.")
			return
		}
		c.run(c.absoluteJobID(blocks[1]))
	case "tasks":
		if len(blocks) == 1 {
			c.Errorf("Argument is missing.")
			return
		}
		c.readTasks(c.absoluteJobID(blocks[1]))
	case "delete":
		if len(blocks) == 1 {
			c.Errorf("Argument is missing.")
			return
		}
		c.deleteJob(c.absoluteJobID(blocks[1]))
	default:
		c.Errorf("Invalid command.")
	}
}

func (c *ClientCommand) prefix() (string, bool) {
	prefix := ""
	if c.group != "" {
		prefix += c.group
	}
	if c.project != "" {
		prefix += "/" + c.project
	}
	return fmt.Sprintf("%s> ", prefix), true
}

func (c *ClientCommand) authReq() func(*http.Request) error {
	return func(req *http.Request) error {
		switch c.auth {
		case "":
			return nil
		case "gitlab":
			req.Header.Add("X-Token", c.token)
		case "ldap":
			req.SetBasicAuth(c.username, c.password)
		default:
			return fmt.Errorf("Unknown authentication method: %s", c.auth)
		}
		return nil
	}
}

func (c *ClientCommand) authInit() error {
	if c.auth == "" {
		if v := os.Getenv(envRhythmAuth); v != "" {
			c.auth = v
		}
	}
	switch c.auth {
	case "":
		return nil
	case "gitlab":
		token, err := c.readGitLabToken()
		if err != nil {
			return err
		}
		if token == "" {
			token, err = c.Ui.AskSecret("GitLab token:")
			if err != nil {
				return err
			}
		}
		c.token = token
	case "ldap":
		username, err := c.Ui.AskSecret("LDAP username:")
		if err != nil {
			return err
		}
		password, err := c.Ui.AskSecret("LDAP password:")
		if err != nil {
			return err
		}
		c.username = username
		c.password = password
	default:
		return fmt.Errorf("Unknown authentication method: %s", c.auth)
	}
	return nil
}

// Run executes a command.
func (c *ClientCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	fmt.Printf("rhythm %s\n", c.Version)
	fmt.Println("Please use `exit` or `Ctrl-D` to exit.")
	c.authInit()
	c.serverCallsThrottler = backoff.Notifier(time.Second*10, time.Second*10, context.Background().Done())
	cli, err := apiclient.New(c.addr, c.authReq())
	if err != nil {
		c.Errorf("Error creating API client: %s", err)
		return 1
	}
	c.apiClient = cli
	jobs, err := cli.FindJobs("")
	if err != nil {
		c.Errorf("Error getting jobs: %s", err)
		return 1
	}
	c.jobs = jobs
	p := prompt.New(
		c.executor,
		c.completer,
		prompt.OptionPrefix("> "),
		prompt.OptionLivePrefix(c.prefix),
	)
	p.Run()
	return 0
}

// Help returns full manual.
func (c *ClientCommand) Help() string {
	help := `
Usage: rhythm client [options]

  Start interactive client.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *ClientCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("client", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *ClientCommand) Synopsis() string {
	return "Start interactive client"
}

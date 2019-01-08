package command

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mitchellh/cli"
	"github.com/mlowicki/rhythm/command/apiclient"
	"github.com/mlowicki/rhythm/command/tokenhelper"
	"github.com/mlowicki/rhythm/model"
)

const (
	envRhythmAuth        = "RHYTHM_AUTH"
	envRhythmTokenHelper = "RHYTHM_TOKEN_HELPER"
)

// BaseCommand implements funcionality shared by all commands.
type BaseCommand struct {
	Ui cli.Ui
}

// Errorf outputs formatted error message.
func (c *BaseCommand) Errorf(format string, a ...interface{}) {
	c.Ui.Error(fmt.Sprintf(format, a...))
}

// Printf outputs formatted message.
func (c *BaseCommand) Printf(format string, a ...interface{}) {
	c.Ui.Output(fmt.Sprintf(format, a...))
}

func (c *BaseCommand) getTokenHelper() (tokenhelper.Helper, error) {
	if path := os.Getenv(envRhythmTokenHelper); path != "" {
		// Absolute path are only allowed to avoid opening arbitrary binary
		// from e.g. current directory.
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("%s must be set to an absolute path", envRhythmTokenHelper)

		}
		return &tokenhelper.External{BinaryPath: path}, nil
	}
	return &tokenhelper.Internal{}, nil
}

func (c *BaseCommand) readGitLabToken() (string, error) {
	helper, err := c.getTokenHelper()
	if err != nil {
		return "", err
	}
	token, err := helper.Read()
	if err != nil {
		return "", fmt.Errorf("Error getting token from token helper: %s", err)
	}
	return token, nil
}

/**
 * Possible methods are "gitlab", "ldap" or "" (blank),
 *
 * If blank is passed then method is read from env var. If env var is not set
 * or empty then no authentication is assumed.
 */
func (c *BaseCommand) authReq(method string) func(*http.Request) error {
	return func(req *http.Request) error {
		if method == "" {
			if v := os.Getenv(envRhythmAuth); v != "" {
				method = v
			}
		}
		switch method {
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
			req.Header.Add("X-Token", token)
		case "ldap":
			username, err := c.Ui.AskSecret("LDAP username:")
			if err != nil {
				return err
			}
			password, err := c.Ui.AskSecret("LDAP password:")
			if err != nil {
				return err
			}
			req.SetBasicAuth(username, password)
		default:
			return fmt.Errorf("Unknown authentication method: %s", method)
		}
		return nil
	}
}

func (c *BaseCommand) printHealth(health *apiclient.HealthInfo) {
	c.Printf("Leader: %t", health.Leader)
	c.Printf("Version: %s", health.Version)
	c.Printf("ServerTime: %s", health.ServerTime)
}

func (c *BaseCommand) printJob(job *model.Job) {
	c.Printf("State: %s", coloredState(job.State))
	if job.State == model.FAILED {
		c.Printf("    Retries: %d", job.Retries)
	}
	if job.LastStart.IsZero() {
		c.Printf("    Last start: Not started yet")
	} else {
		c.Printf("    Last start: %s", job.LastStart.Format(time.UnixDate))
	}
	c.Printf("    Max retries: %d", job.MaxRetries)
	if t := job.Schedule.Type; t != model.Cron {
		c.Printf("Unknown schedule type: %s", t)
	}
	c.Printf("Scheduler: Cron")
	c.Printf("    Rule: %s", job.Schedule.Cron)
	c.Printf("    Next start: %s", job.NextRun().Format(time.UnixDate))
	switch job.Container.Type {
	case model.Mesos:
		c.Printf("Container: Mesos")
		c.Printf("    Image: %s", job.Container.Mesos.Image)
	case model.Docker:
		c.Printf("Container: Docker")
		c.Printf("    Image: %s", job.Container.Docker.Image)
		c.Printf("    Force pull image: %t", job.Container.Docker.ForcePullImage)
	}
	if job.Shell {
		c.Printf("Cmd: /bin/sh -c '%s'", job.Cmd)
	} else {
		c.Printf("Cmd: %s %s", job.Cmd, strings.Join(job.Arguments, " "))
	}
	c.printMap("Environment", job.Env)
	c.printMap("Secrets", job.Secrets)
	c.Printf("User: %s", job.User)
	c.Printf("Resources:")
	c.Printf("    Memory: %.1f MB", job.Mem)
	c.Printf("    Disk: %.1f MB", job.Disk)
	c.Printf("    CPUs: %.1f", job.CPUs)
	c.printMap("Labels", job.Labels)
}

func (c *BaseCommand) printTasks(tasks []*model.Task) {
	for i, task := range tasks {
		if i > 0 {
			c.Printf("")
		}
		var status string
		if task.Source == "" {
			status = color.GreenString("SUCCESS")
		} else {
			status = color.RedString("FAIL")
		}
		c.Printf("Status: \t%s", status)
		c.Printf("Start: \t\t%s", task.Start.Format(time.UnixDate))
		c.Printf("End: \t\t%s", task.Start.Format(time.UnixDate))
		if task.TaskID != "" {
			c.Printf("Task ID: \t%s", task.TaskID)
		}
		if task.ExecutorID != "" {
			c.Printf("Executor ID: \t%s", task.ExecutorID)
		}
		if task.AgentID != "" {
			c.Printf("Agent ID: \t%s", task.AgentID)
		}
		if task.FrameworkID != "" {
			c.Printf("Framework ID: \t%s", task.FrameworkID)
		}
		if task.ExecutorURL != "" {
			c.Printf("Executor URL: \t%s", task.ExecutorURL)
		}
		if task.Message != "" {
			c.Printf("Message: \t%s", task.Message)
		}
		if task.Reason != "" {
			c.Printf("Reason: \t%s", task.Reason)
		}
		if task.Source != "" {
			c.Printf("Source: \t%s", task.Source)
		}
	}
}

func (c *BaseCommand) printMap(title string, m map[string]string) {
	if len(m) == 0 {
		return
	}
	c.Printf("%s:", title)
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c.Printf("    %s: %s", k, m[k])
	}
}

var stateColorFunc = map[model.State]func(format string, a ...interface{}) string{
	model.IDLE:    color.GreenString,
	model.RUNNING: color.YellowString,
	model.FAILED:  color.RedString,
}

func coloredState(state model.State) string {
	fun, ok := stateColorFunc[state]
	if ok {
		return fun(state.String())
	}
	return state.String()
}

type flagSet struct {
	*flag.FlagSet
}

func (fs *flagSet) help() string {
	var buf bytes.Buffer
	fs.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(&buf, "  -%s\n", f.Name)
		fmt.Fprintf(&buf, "      %s", f.Usage)
		if f.DefValue != "" {
			fmt.Fprintf(&buf, " (default: %s)", f.DefValue)
		}
		fmt.Fprint(&buf, "\n\n")
	})
	return buf.String()
}

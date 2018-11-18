package command

import (
	"flag"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mlowicki/rhythm/command/apiclient"
)

type ReadTasksCommand struct {
	*BaseCommand
	addr string
	auth string
}

func (c *ReadTasksCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (fully-qualified job ID)")
		return 1
	}
	tasks, err := apiclient.New(c.addr, c.authReq(c.auth)).ReadTasks(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
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
	return 0
}

func (c *ReadTasksCommand) Help() string {
	help := `
Usage: rhythm read-tasks [options] FQID

  Show tasks (runs) of job with given fully-qualified ID (e.g. "group/project/id").

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *ReadTasksCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("read-tasks", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

func (c *ReadTasksCommand) Synopsis() string {
	return "Show job's tasks (runs)"
}

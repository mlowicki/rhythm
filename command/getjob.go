package command

import (
	"flag"
	"sort"
	"strings"
	"time"

	"github.com/mlowicki/rhythm/command/apiclient"
	"github.com/mlowicki/rhythm/model"
)

type GetJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

func (c *GetJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (fully-qualified job ID)")
		return 1
	}
	fqid := fs.Args()[0]
	job, err := apiclient.New(c.addr, c.authReq(c.auth)).GetJob(fqid)
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	c.Printf("State: %s", coloredState(job.State))
	if job.State == model.FAILED {
		c.Printf("    Retries: %d out of %d", job.Retries, job.MaxRetries)
	}
	if job.LastStart.IsZero() {
		c.Printf("    Last start: Not started yet")
	} else {
		c.Printf("    Last start: %s", job.LastStart.Format(time.UnixDate))
	}
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
	return 0
}

func (c *GetJobCommand) printMap(title string, m map[string]string) {
	if len(m) > 0 {
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
}

func (c *GetJobCommand) Help() string {
	help := `
Usage: rhythm get-job [options] FQID

  Show configuration and current state of job with the given fully-qualified ID (e.g. "group/project/id").

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *GetJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("get-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

func (c *GetJobCommand) Synopsis() string {
	return "Show job configuration and current state"
}

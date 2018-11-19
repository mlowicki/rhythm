package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

type FindJobsCommand struct {
	*BaseCommand
	addr string
	auth string
}

func (c *FindJobsCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) > 1 {
		c.Errorf("Zero or one arugment is allowed")
		return 1
	}
	var filter string
	if len(args) == 1 {
		filter = args[0]
	}
	jobs, err := apiclient.New(c.addr, c.authReq(c.auth)).FindJobs(filter)
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	for _, job := range jobs {
		c.Printf("%s/%s/%s", job.Group, job.Project, job.ID)
	}
	return 0
}

func (c *FindJobsCommand) Help() string {
	help := `
Usage: rhythm find-jobs [options] FILTER

  Show IDs of jobs matching FILTER.

  FILTER can be one of:
  * GROUP to return all jobs from group
  * GROUP/PROJECT to return all jobs from project
  * no set to return all jobs across all groups and projects

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *FindJobsCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("find-jobs", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

func (c *FindJobsCommand) Synopsis() string {
	return "Show IDs of jobs matching filter"
}
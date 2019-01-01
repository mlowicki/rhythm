package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// FindJobsCommand implements command for searching based on specified pattern.
type FindJobsCommand struct {
	*BaseCommand
	addr      string
	auth      string
	showState bool
}

// Run executes a command.
func (c *FindJobsCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) > 1 {
		c.Errorf("Zero or one argument is allowed")
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
		state := ""
		if c.showState {
			state = coloredState(job.State)
		}
		c.Printf("%s/%s/%s %s", job.Group, job.Project, job.ID, state)
	}
	return 0
}

// Help returns full manual.
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

// Flags returns parameters associated with command.
func (c *FindJobsCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("find-jobs", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	fs.BoolVar(&c.showState, "state", true, "Show job state")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *FindJobsCommand) Synopsis() string {
	return "Show IDs of jobs matching filter"
}

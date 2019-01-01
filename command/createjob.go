package command

import (
	"flag"
	"io/ioutil"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// CreateJobCommand implements command for adding new job.
type CreateJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

// Run executes a command.
func (c *CreateJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (path to job config)")
		return 1
	}
	jobEncoded, err := ioutil.ReadFile(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	err = apiclient.New(c.addr, c.authReq(c.auth)).CreateJob(jobEncoded)
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

// Help returns full manual.
func (c *CreateJobCommand) Help() string {
	help := `
Usage: rhythm create-job [options] PATH

  Add new job specified by config file located under PATH.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *CreateJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("create-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *CreateJobCommand) Synopsis() string {
	return "Add new job"
}

package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// RunJobCommand implements command for scheduling job for immediate run.
type RunJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

// Run executes a command.
func (c *RunJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (fully-qualified job ID)")
		return 1
	}
	cli, err := apiclient.New(c.addr, c.authReq(c.auth))
	if err != nil {
		c.Errorf("Error creating API client: %s", err)
		return 1
	}
	err = cli.RunJob(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

// Help returns full manual.
func (c *RunJobCommand) Help() string {
	help := `
Usage: rhythm run-job [options] FQID

  Schedule job with the given fully-qualified ID (e.g. "group/project/id") for immediate run.
  If job is already queued (scheduled but not launched yet) then command will be no-op.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *RunJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("run-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *RunJobCommand) Synopsis() string {
	return "Run job"
}

package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// ReadJobCommand implements command for getting job's info.
type ReadJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

// Run executes a command.
func (c *ReadJobCommand) Run(args []string) int {
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
	job, err := cli.ReadJob(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	c.printJob(job)
	return 0
}

// Help returns full manual.
func (c *ReadJobCommand) Help() string {
	help := `
Usage: rhythm read-job [options] FQID

  Show configuration and state of job with the given fully-qualified ID (e.g. "group/project/id").

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *ReadJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("read-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *ReadJobCommand) Synopsis() string {
	return "Show job configuration and state"
}

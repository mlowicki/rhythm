package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// DeleteJobCommand implements command for deleting existing job.
type DeleteJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

// Run executes a command.
func (c *DeleteJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (fully-qualified job ID)")
		return 1
	}
	err := apiclient.New(c.addr, c.authReq(c.auth)).DeleteJob(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

// Help returns full manual.
func (c *DeleteJobCommand) Help() string {
	help := `
Usage: rhythm delete-job [options] FQID

  Remove job with the given fully-qualified ID (e.g. "group/project/id").

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *DeleteJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("delete-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *DeleteJobCommand) Synopsis() string {
	return "Remove job"
}

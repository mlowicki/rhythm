package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

type RunJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

func (c *RunJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (fully-qualified job ID)")
		return 1
	}
	err := apiclient.New(c.addr, c.authReq(c.auth)).RunJob(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

func (c *RunJobCommand) Help() string {
	help := `
Usage: rhythm run-job [options] FQID

  Schedule job with the given fully-qualified ID (e.g. "group/project/id") for immediate run.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *RunJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("run-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

func (c *RunJobCommand) Synopsis() string {
	return "Run job"
}

package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

type DeleteJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

func (c *DeleteJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 1 {
		c.Errorf("Exactly one argument is required (fully-qualified job ID)")
		return 1
	}
	fqid := fs.Args()[0]
	err := apiclient.New(c.addr, c.authReq(c.auth)).DeleteJob(fqid)
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

func (c *DeleteJobCommand) Help() string {
	help := `
Usage: rhythm delete-job [options] FQID

  Delete job with the given fully-qualified ID (e.g. "group/project/id").

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *DeleteJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("get-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

func (c *DeleteJobCommand) Synopsis() string {
	return "Show job configuration and current state"
}

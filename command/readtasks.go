package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// ReadTasksCommand implements command for returning job's run (history).
type ReadTasksCommand struct {
	*BaseCommand
	addr string
	auth string
}

// Run executes a command.
func (c *ReadTasksCommand) Run(args []string) int {
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
	tasks, err := cli.ReadTasks(args[0])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	c.printTasks(tasks)
	return 0
}

// Help returns full manual.
func (c *ReadTasksCommand) Help() string {
	help := `
Usage: rhythm read-tasks [options] FQID

  Show tasks (runs) of job with given fully-qualified ID (e.g. "group/project/id").

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *ReadTasksCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("read-tasks", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *ReadTasksCommand) Synopsis() string {
	return "Show job's tasks (runs)"
}

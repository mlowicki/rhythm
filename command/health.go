package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

// HealthCommand implements command for returning server's info.
type HealthCommand struct {
	*BaseCommand
	addr string
}

// Run executes a command.
func (c *HealthCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	cli, err := apiclient.New(c.addr, nil)
	if err != nil {
		c.Errorf("Error creating API client: %s", err)
		return 1
	}
	health, err := cli.Health()
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	c.printHealth(health)
	return 0
}

// Help returns full manual.
func (c *HealthCommand) Help() string {
	help := `
Usage: rhythm health [options]

  Show status of Rhythm server.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *HealthCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *HealthCommand) Synopsis() string {
	return "Show status of Rhythm server"
}

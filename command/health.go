package command

import (
	"flag"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

type HealthCommand struct {
	*BaseCommand
	addr string
}

func (c *HealthCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	health, err := apiclient.New(c.addr, c.authReq("")).Health()
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	c.Printf("Leader: %t", health.Leader)
	c.Printf("Version: %s", health.Version)
	c.Printf("ServerTime: %s", health.ServerTime)
	return 0
}

func (c *HealthCommand) Help() string {
	help := `
Usage: rhythm health [options]

  Show status of Rhythm server.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *HealthCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	return &flagSet{fs}
}

func (c *HealthCommand) Synopsis() string {
	return "Show status of Rhythm server"
}

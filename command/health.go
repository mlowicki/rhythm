package command

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"strings"
)

type HealthCommand struct {
	*BaseCommand
	addr string
}

func (c *HealthCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	u, err := c.getAddr(c.addr)
	if err != nil {
		c.Errorf(err.Error())
		return 1
	}
	u.Path = "api/v1/health"
	resp, err := http.Get(u.String())
	if err != nil {
		c.Errorf("Error getting server status: %s", err)
		return 1
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.Errorf("Error reading response body: %s", err)
		return 1
	}
	if resp.StatusCode != http.StatusOK {
		c.Errorf("Server error: %d", resp.StatusCode)
		c.Errorf("Response:\n%s", body)
		return 1
	}
	var health struct {
		Leader     bool
		Version    string
		ServerTime string
	}
	err = json.Unmarshal(body, &health)
	if err != nil {
		c.Errorf("Error decoding server status: %s", err)
		c.Errorf("Response:\n%s", body)
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

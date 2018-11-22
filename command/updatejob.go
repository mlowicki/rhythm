package command

import (
	"flag"
	"io/ioutil"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
)

type UpdateJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

func (c *UpdateJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	if len(args) != 2 {
		c.Errorf("Exactly two arguments are required (fully-qualified job ID and path to job config)")
		return 1
	}
	changesEncoded, err := ioutil.ReadFile(args[1])
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	err = apiclient.New(c.addr, c.authReq(c.auth)).UpdateJob(args[0], changesEncoded)
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

func (c *UpdateJobCommand) Help() string {
	help := `
Usage: rhythm update-job [options] FQID PATH

  Modify job specified by given fully-qualified ID (e.g. "group/project/id") with config file located under PATH.
  Only parameters form config file will be changed - absent parameters wont' be modified.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *UpdateJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("update-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

func (c *UpdateJobCommand) Synopsis() string {
	return "Modify job"
}

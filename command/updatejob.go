package command

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"strings"

	"github.com/mlowicki/rhythm/command/apiclient"
	"github.com/mlowicki/rhythm/model"
)

// UpdateJobCommand implements command for changing existing job.
type UpdateJobCommand struct {
	*BaseCommand
	addr string
	auth string
}

// Run executes a command.
func (c *UpdateJobCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	args = fs.Args()
	var path string
	var fqid string
	if len(args) == 1 {
		path = args[0]
	} else if len(args) == 2 {
		fqid = args[0]
		path = args[1]
	} else {
		c.Errorf("One argument (path to job config) or two (fully-qualified job ID and path to job config) are required.")
		return 1
	}
	updates, err := ioutil.ReadFile(path)
	if err != nil {
		c.Errorf("Errorf reading config file: %s", err)
		return 1
	}
	if len(args) == 1 {
		var jid model.JobID
		var err = json.Unmarshal(updates, &jid)
		if err != nil {
			c.Errorf("Error decoding config file: %s", err)
			return 1
		}
		if jid.Group == "" || jid.Project == "" || jid.ID == "" {
			c.Errorf("If only path to job config is passed then config must contain group, project and ID.")
			return 1
		}
		fqid = jid.Path()
	}
	err = apiclient.New(c.addr, c.authReq(c.auth)).UpdateJob(fqid, updates)
	if err != nil {
		c.Errorf("%s", err)
		return 1
	}
	return 0
}

// Help returns full manual.
func (c *UpdateJobCommand) Help() string {
	help := `
Usage: rhythm update-job [options] FQID PATH

  Modify job specified by given fully-qualified ID (e.g. "group/project/id") with config file located under PATH.
  Only parameters form config file will be changed - absent parameters wont' be modified.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

// Flags returns parameters associated with command.
func (c *UpdateJobCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("update-job", flag.ContinueOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	fs.StringVar(&c.addr, "addr", "", "Address of Rhythm server (with protocol e.g. \"https://example.com\")")
	fs.StringVar(&c.auth, "auth", "", "Authentication method (\"ldap\" or \"gitlab\")")
	return &flagSet{fs}
}

// Synopsis returns short, one-line help.
func (c *UpdateJobCommand) Synopsis() string {
	return "Modify job"
}

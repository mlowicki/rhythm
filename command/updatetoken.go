package command

import (
	"flag"
	"strings"
)

type UpdateTokenCommand struct {
	*BaseCommand
}

func (c *UpdateTokenCommand) Run(args []string) int {
	fs := c.Flags()
	fs.Parse(args)
	helper, err := c.getTokenHelper()
	if err != nil {
		c.Errorf("Error getting token helper: %s", err)
		return 1
	}
	token, err := c.Ui.AskSecret("Token:")
	if err != nil {
		c.Errorf("Error reading token: %s", err)
		return 1
	}
	err = helper.Update(token)
	if err != nil {
		c.Errorf("Error updating token: %s", err)
		return 1
	}
	return 0
}

func (c *UpdateTokenCommand) Help() string {
	help := `
Usage: rhythm update-token

  Update (or set) authz token.

` + c.Flags().help()
	return strings.TrimSpace(help)
}

func (c *UpdateTokenCommand) Flags() *flagSet {
	fs := flag.NewFlagSet("update-token", flag.ExitOnError)
	fs.Usage = func() { c.Printf(c.Help()) }
	return &flagSet{fs}
}

func (c *UpdateTokenCommand) Synopsis() string {
	return "Update (or set) authz token"
}

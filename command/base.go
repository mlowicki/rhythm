package command

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"

	"github.com/fatih/color"
	"github.com/mitchellh/cli"
	"github.com/mlowicki/rhythm/model"
)

type BaseCommand struct {
	Ui cli.Ui
}

func (c *BaseCommand) Errorf(format string, a ...interface{}) {
	c.Ui.Error(fmt.Sprintf(format, a...))
}

func (c *BaseCommand) Printf(format string, a ...interface{}) {
	c.Ui.Output(fmt.Sprintf(format, a...))
}

func (c *BaseCommand) authReq(method string) func(*http.Request) error {
	return func(req *http.Request) error {
		switch method {
		case "":
		case "gitlab":
			token, err := c.Ui.AskSecret("GitLab token:")
			if err != nil {
				return err
			}
			req.Header.Add("X-Token", token)
		case "ldap":
			username, err := c.Ui.AskSecret("LDAP username:")
			if err != nil {
				return err
			}
			password, err := c.Ui.AskSecret("LDAP password:")
			if err != nil {
				return err
			}
			req.SetBasicAuth(username, password)
		default:
			return fmt.Errorf("Unknown authentication method: %s", method)
		}
		return nil
	}
}

var stateColorFunc = map[model.State]func(format string, a ...interface{}) string{
	model.IDLE:    color.GreenString,
	model.RUNNING: color.YellowString,
	model.FAILED:  color.RedString,
}

func coloredState(state model.State) string {
	fun, ok := stateColorFunc[state]
	if ok {
		return fun(state.String())
	} else {
		return state.String()
	}
}

type flagSet struct {
	*flag.FlagSet
}

func (fs *flagSet) help() string {
	var buf bytes.Buffer
	fs.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(&buf, "  -%s\n", f.Name)
		fmt.Fprintf(&buf, "      %s", f.Usage)
		if f.DefValue != "" {
			fmt.Fprintf(&buf, " (default: %s)", f.DefValue)
		}
		fmt.Fprint(&buf, "\n\n")
	})
	return buf.String()
}
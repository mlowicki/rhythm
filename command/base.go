package command

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/mitchellh/cli"
	"github.com/mlowicki/rhythm/command/tokenhelper"
	"github.com/mlowicki/rhythm/model"
)

const (
	envRhythmAuth        = "RHYTHM_AUTH"
	envRhythmTokenHelper = "RHYTHM_TOKEN_HELPER"
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

func (c *BaseCommand) getTokenHelper() (tokenhelper.Helper, error) {
	if path := os.Getenv(envRhythmTokenHelper); path != "" {
		// Absolute path are only allowed to avoid opening arbitrary binary
		// from e.g. current directory.
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("%s must be set to an absolute path", envRhythmTokenHelper)

		}
		return &tokenhelper.External{BinaryPath: path}, nil
	}
	return &tokenhelper.Internal{}, nil
}

func (c *BaseCommand) readGitLabToken() (string, error) {
	helper, err := c.getTokenHelper()
	if err != nil {
		return "", err
	}
	token, err := helper.Read()
	if err != nil {
		return "", fmt.Errorf("Error getting token from token helper: %s", err)
	}
	return token, nil
}

/**
 * Possible methods are "gitlab", "ldap" or "" (blank),
 *
 * If blank is passed then method is read from env var. If env var is not set
 * or empty then no authentication is assumed.
 */
func (c *BaseCommand) authReq(method string) func(*http.Request) error {
	return func(req *http.Request) error {
		if method == "" {
			if v := os.Getenv(envRhythmAuth); v != "" {
				method = v
			}
		}
		switch method {
		case "":
			return nil
		case "gitlab":
			token, err := c.readGitLabToken()
			if err != nil {
				return err
			}
			if token == "" {
				token, err = c.Ui.AskSecret("GitLab token:")
				if err != nil {
					return err
				}
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
	}
	return state.String()
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

package command

import (
	"bytes"
	"flag"
	"fmt"
	"net/url"
	"os"

	"github.com/fatih/color"
	"github.com/mitchellh/cli"
	"github.com/mlowicki/rhythm/model"
)

const envRhythmAddr = "RHYTHM_ADDR"

type BaseCommand struct {
	Ui cli.Ui
}

func (c *BaseCommand) Errorf(format string, a ...interface{}) {
	c.Ui.Error(fmt.Sprintf(format, a...))
}

func (c *BaseCommand) Printf(format string, a ...interface{}) {
	c.Ui.Output(fmt.Sprintf(format, a...))
}

func (c *BaseCommand) getAddr(flagAddr string) (*url.URL, error) {
	var addr string
	if v := os.Getenv(envRhythmAddr); v != "" {
		addr = v
	}
	if flagAddr != "" {
		addr = flagAddr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("Failed parsing server address: %s", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Invalid server address scheme: %s", u.Scheme)
	}
	return u, nil
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

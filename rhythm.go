package main

import (
	"os"

	"github.com/mitchellh/cli"
	"github.com/mlowicki/rhythm/command"
	"github.com/onrik/logrus/filename"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.AddHook(filename.NewHook())
}

const version = "0.13"

func main() {
	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}
	coloredUi := &cli.ColoredUi{
		Ui:         ui,
		ErrorColor: cli.UiColorRed,
	}
	c := cli.NewCLI("rhythm", version)
	c.Args = os.Args[1:]
	baseCmd := command.BaseCommand{Ui: coloredUi}
	c.Commands = map[string]cli.CommandFactory{
		"server": func() (cli.Command, error) {
			return &command.ServerCommand{BaseCommand: &baseCmd, Version: version}, nil
		},
		"health": func() (cli.Command, error) {
			return &command.HealthCommand{BaseCommand: &baseCmd}, nil
		},
		"create-job": func() (cli.Command, error) {
			return &command.CreateJobCommand{BaseCommand: &baseCmd}, nil
		},
		"update-job": func() (cli.Command, error) {
			return &command.UpdateJobCommand{BaseCommand: &baseCmd}, nil
		},
		"read-job": func() (cli.Command, error) {
			return &command.ReadJobCommand{BaseCommand: &baseCmd}, nil
		},
		"delete-job": func() (cli.Command, error) {
			return &command.DeleteJobCommand{BaseCommand: &baseCmd}, nil
		},
		"run-job": func() (cli.Command, error) {
			return &command.RunJobCommand{BaseCommand: &baseCmd}, nil
		},
		"find-jobs": func() (cli.Command, error) {
			return &command.FindJobsCommand{BaseCommand: &baseCmd}, nil
		},
		"read-tasks": func() (cli.Command, error) {
			return &command.ReadTasksCommand{BaseCommand: &baseCmd}, nil
		},
		"update-token": func() (cli.Command, error) {
			return &command.UpdateTokenCommand{BaseCommand: &baseCmd}, nil
		},
	}
	exitStatus, err := c.Run()
	if err != nil {
		log.Error(err)
	}
	os.Exit(exitStatus)
}

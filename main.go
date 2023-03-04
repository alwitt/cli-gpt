package main

import (
	"os"

	"github.com/alwitt/cli-gpt/cmd"
	"github.com/apex/log"
	"github.com/urfave/cli/v2"
)

func main() {
	logTags := log.Fields{
		"module":    "main",
		"component": "main",
	}

	app := &cli.App{
		Version:     "v0.1.0",
		Usage:       "application entrypoint",
		Description: "OpenAI API CLI application",
		// Components
		Commands: []*cli.Command{
			{
				Name:        "get",
				Usage:       "Read resources",
				Description: "Fetch recorded resources",
				Subcommands: cmd.GenerateGetSubcommands(),
			},
			{
				Name:        "create",
				Usage:       "Create resources",
				Description: "Create new resources",
				Subcommands: cmd.GenerateCreateSubcommands(),
			},
			{
				Name:        "delete",
				Usage:       "Delete resources",
				Description: "Delete resources",
				Subcommands: cmd.GenerateDeleteSubcommands(),
			},
			{
				Name:        "context",
				Usage:       "User context settings",
				Description: "User context settings",
				Subcommands: cmd.GenerateContextSubcommands(),
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.WithError(err).WithFields(logTags).Fatal("Program shutdown")
	}
}

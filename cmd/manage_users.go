package cmd

import (
	"fmt"

	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// interactiveUserSelection interactive way to select a user
func interactiveUserSelection(app *applicationContext) (string, error) {
	logtags := app.GetLogTagsForContext(app.ctxt)

	allUsers, err := app.userManager.ListUsers(app.ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Unable to query all users")
		return "", err
	}
	if len(allUsers) == 0 {
		return "", fmt.Errorf("no users registered with the system")
	}

	type userDisplay struct {
		UserID   string
		Username string
	}
	displayEntries := []userDisplay{}
	usernames := []string{}

	for _, oneUser := range allUsers {
		userID, err := oneUser.GetID(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("User ID read failed")
			return "", err
		}
		username, err := oneUser.GetName(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Username read failed")
			return "", err
		}
		displayEntries = append(displayEntries, userDisplay{UserID: userID, Username: username})
		usernames = append(usernames, username)
	}

	userPrompt := promptui.Select{Label: "Select user", Items: usernames}
	selected, _, err := userPrompt.Run()
	if err != nil {
		return "", err
	}

	return displayEntries[selected].UserID, nil
}

// ================================================================================

/*
actionCreateUser create new user

	@param args *commonCLIArgs - CLI arguments
	@return the CLI action
*/
func actionCreateUser(args *commonCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "create-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		// Prompt for user info
		usernamePrompt := promptui.Prompt{Label: "Username"}
		username, err := usernamePrompt.Run()
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to query for username")
			return err
		}

		apiTokenPrompt := promptui.Prompt{Label: "API Token"}
		apiToken, err := apiTokenPrompt.Run()
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to query for API token")
			return err
		}

		userEntry, err := app.userManager.RecordNewUser(app.ctxt, username)
		if err != nil {
			log.WithError(err).WithFields(logtags).Errorf("Failed to define new user '%s'", username)
			return nil
		}

		// Install user API token
		if err := userEntry.SetAPIToken(app.ctxt, apiToken); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Failed to record API token to user '%s'", username)
			return nil
		}

		log.WithFields(logtags).Infof("Created new user '%s'", username)

		return nil
	}
}

// ================================================================================

/*
actionListUsers list available users

	@param args *commonCLIArgs - CLI arguments
	@return the CLI action
*/
func actionListUsers(args *commonCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		userEntries, err := app.userManager.ListUsers(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to list all user entires")
			return nil
		}

		if len(userEntries) > 0 {
			// Print the user entries
			type userDisplay struct {
				UserID   string `yaml:"id"`
				Username string `yaml:"username"`
			}
			displayEntries := []userDisplay{}
			for _, oneUser := range userEntries {
				userID, err := oneUser.GetID(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read user ID")
					return err
				}
				username, err := oneUser.GetName(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read username")
					return err
				}
				displayEntries = append(displayEntries, userDisplay{UserID: userID, Username: username})
			}

			type toDisplay struct {
				ActiveUser userDisplay   `yaml:"active"`
				AllUsers   []userDisplay `yaml:"users"`
			}
			display := toDisplay{AllUsers: displayEntries}
			if app.currentUser != nil {
				userID, err := app.currentUser.GetID(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read active user ID")
					return err
				}
				username, err := app.currentUser.GetName(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read active username")
					return err
				}
				display.ActiveUser = userDisplay{UserID: userID, Username: username}
			}

			// Display as YAML
			t, _ := yaml.Marshal(&display)

			fmt.Printf("%s\n", t)
		}

		return nil
	}
}

// ================================================================================

// specifyUserCLIArgs cli arguments needed to refer to a user
type specifyUserCLIArgs struct {
	commonCLIArgs
	// UserID user ID to delete
	UserID string
}

/*
getCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *specifyUserCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.GetCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "user-id",
			Usage:       "ID of user to set as active",
			Aliases:     []string{"i"},
			EnvVars:     []string{"USER_ID"},
			Destination: &c.UserID,
			Required:    false,
		},
	}...)

	return cliFlags
}

var specifyUserParams specifyUserCLIArgs

/*
actionChangeActiveUser change the active user

	@param args *changeActiveUserCLIArgs - CLI arguments
	@return the CLI action
*/
func actionChangeActiveUser(args *specifyUserCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "create-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if args.UserID == "" {
			args.UserID, err = interactiveUserSelection(app)
			if err != nil {
				log.WithError(err).WithFields(logtags).Error("User selection failure")
				return err
			}
		}

		userEntry, err := app.userManager.GetUser(app.ctxt, args.UserID)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Could not find user entry with ID '%s'", args.UserID)
			return err
		}

		app.currentUser = userEntry
		if err := app.record(); err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to record application context")
			return err
		}

		return nil
	}
}

// ================================================================================

/*
actionDeleteUser delete a user

	@param args *specifyUserCLIArgs - CLI arguments
	@return the CLI action
*/
func actionDeleteUser(args *specifyUserCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "delete-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if args.UserID == "" {
			args.UserID, err = interactiveUserSelection(app)
			if err != nil {
				log.WithError(err).WithFields(logtags).Error("User selection failure")
				return err
			}
		}

		if app.currentUser != nil {
			currentUserID, err := app.currentUser.GetID(app.ctxt)
			if err != nil || currentUserID == args.UserID {
				app.currentUser = nil
				if err := app.record(); err != nil {
					log.WithError(err).WithFields(logtags).Error("Unable to record application context")
					return err
				}
			}
		}

		if err := app.userManager.DeleteUser(app.ctxt, args.UserID); err != nil {
			log.WithError(err).WithFields(logtags).Errorf("Failed to delete user '%s'", args.UserID)
			return err
		}

		return nil
	}
}

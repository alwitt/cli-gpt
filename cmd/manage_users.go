package cmd

import (
	"fmt"

	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// ================================================================================

// createUserCLIArgs cli arguments needed defining a new user
type createUserCLIArgs struct {
	commonCLIArgs
	// Username new user name
	Username string `validate:"required"`
	// APIToken user's API token
	APIToken string `validate:"required"`
}

/*
getCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *createUserCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.getCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "username",
			Usage:       "New user name",
			Aliases:     []string{"u"},
			EnvVars:     []string{"NEW_USERNAME"},
			Destination: &c.Username,
			Required:    true,
		},
		&cli.StringFlag{
			Name:        "api-token",
			Usage:       "User's API token",
			Aliases:     []string{"t"},
			EnvVars:     []string{"USER_API_TOKEN"},
			Destination: &c.APIToken,
			Required:    true,
		},
	}...)

	return cliFlags
}

var createUserParams createUserCLIArgs

/*
actionCreateUser create new user

	@param args *createUserCLIArgs - CLI arguments
	@return the CLI action
*/
func actionCreateUser(args *createUserCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "create-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		userEntry, err := app.userManager.RecordNewUser(app.ctxt, args.Username)
		if err != nil {
			log.WithError(err).WithFields(logtags).Errorf("Failed to define new user '%s'", args.Username)
			return nil
		}

		// Install user API token
		if err := userEntry.SetAPIToken(app.ctxt, args.APIToken); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Failed to record API token to user '%s'", args.Username)
			return nil
		}

		log.WithFields(logtags).Infof("Created new user '%s'", args.Username)

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
				UserID   string `yaml:"user-id"`
				Username string `yaml:"username"`
			}
			displayEntries := []userDisplay{}
			for _, oneUser := range userEntries {
				userID, err := oneUser.GetID(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read user ID")
					return nil
				}
				username, err := oneUser.GetName(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read username")
					return nil
				}
				displayEntries = append(displayEntries, userDisplay{UserID: userID, Username: username})
			}

			type toDisplay struct {
				ActiveUser userDisplay   `yaml:"active"`
				AllUsers   []userDisplay `yaml:"all-users"`
			}
			display := toDisplay{AllUsers: displayEntries}
			if app.currentUser != nil {
				userID, err := app.currentUser.GetID(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read active user ID")
					return nil
				}
				username, err := app.currentUser.GetName(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Failed to read active username")
					return nil
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

// changeActiveUserCLIArgs cli arguments needed to change the active user
type changeActiveUserCLIArgs struct {
	commonCLIArgs
	// UserID user ID to delete
	UserID string `validate:"required"`
}

/*
getCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *changeActiveUserCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.getCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "user-id",
			Usage:       "ID of user to set as active",
			Aliases:     []string{"i"},
			EnvVars:     []string{"USER_ID"},
			Destination: &c.UserID,
			Required:    true,
		},
	}...)

	return cliFlags
}

var changeActiveUserParams changeActiveUserCLIArgs

/*
actionChangeActiveUser change the active user

	@param args *changeActiveUserCLIArgs - CLI arguments
	@return the CLI action
*/
func actionChangeActiveUser(args *changeActiveUserCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "create-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

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
actionListUsers list available users

	@param args *commonCLIArgs - CLI arguments
	@return the CLI action
*/
func actionDeleteActiveUser(args *commonCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "delete-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			// No action to take if no active user selected
			err := fmt.Errorf("no active user")
			log.WithError(err).WithFields(logtags).Error("Unable to delete active user")
			return err
		}

		userID, err := app.currentUser.GetID(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to read active user ID")
			return nil
		}

		if err := app.userManager.DeleteUser(app.ctxt, userID); err != nil {
			log.WithError(err).WithFields(logtags).Errorf("Failed to delete user '%s'", userID)
			return err
		}

		app.currentUser = nil
		if err := app.record(); err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to record application context")
			return err
		}

		return nil
	}
}

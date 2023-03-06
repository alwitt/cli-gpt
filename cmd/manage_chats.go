package cmd

import (
	"fmt"
	"os"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// ================================================================================

// startNewChatActionCLIArgs standard cli arguments when starting a new chat session
type startNewChatActionCLIArgs struct {
	commonCLIArgs
	// Model model to use
	Model string `validate:"required,oneof=davinci curie babbage ada"`
	// SettingFile chat session setting file
	SettingFile string `validate:"omitempty,file"`
}

func (c *startNewChatActionCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.GetCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "model",
			Usage:       "Text generation model: [davinci curie babbage ada]",
			Aliases:     []string{"m"},
			EnvVars:     []string{"TEXT_COMPLETION_MODEL"},
			Value:       "davinci",
			DefaultText: "davinci",
			Destination: &c.Model,
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "settings-file",
			Usage:       "YAML file containing chat session settings",
			Aliases:     []string{"f"},
			EnvVars:     []string{"CHAT_SESSION_SETTINGS_FILE"},
			Value:       "",
			DefaultText: "",
			Destination: &c.SettingFile,
			Required:    false,
		},
	}...)

	return cliFlags
}

var startNewChatParams startNewChatActionCLIArgs

/*
actionStartNewChat start a new chat session

	@param args *startNewChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionStartNewChat(args *startNewChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
			return err
		}

		// Parse the new chat setting config
		var newSetting persistence.ChatSessionParameters
		providedSetting := false
		if len(args.SettingFile) > 0 {
			settingContent, err := os.ReadFile(args.SettingFile)
			if err != nil {
				log.
					WithError(err).
					WithFields(logtags).
					Errorf("Unable to read setting file '%s'", args.SettingFile)
				return err
			}
			if err := yaml.Unmarshal(settingContent, &newSetting); err != nil {
				log.
					WithError(err).
					WithFields(logtags).
					Errorf("Unable to parse setting file '%s'", args.SettingFile)
				return err
			}
			providedSetting = true
		}

		// Create new chat session
		session, err := chatManager.NewSession(app.ctxt, args.Model)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to start new chat session")
			return err
		}
		sessionID, err := session.SessionID(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to read session ID of new session")
			return err
		}

		if providedSetting {
			// Get current settings
			currentSetting, err := session.Settings(app.ctxt)
			if err != nil {
				log.
					WithError(err).
					WithFields(logtags).
					Errorf("Unable to read chat session '%s' settings", sessionID)
				return err
			}

			// Merge the new setting into the existing setting
			currentSetting.MergeWithNewSettings(newSetting)

			// Store the updated setting
			if err := session.ChangeSettings(app.ctxt, currentSetting); err != nil {
				log.WithError(err).WithFields(logtags).Error("Failed to apply new session setting")
				return err
			}
		}

		// Make the first exchange

		return nil
	}
}

// ================================================================================

/*
actionListChatSession list chat sessions associated with the active user

	@param args *commonCLIArgs - CLI arguments
	@return the CLI action
*/
func actionListChatSession(args *commonCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
			return err
		}

		sessions, err := chatManager.ListSessions(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to list user's chat sessions")
		}

		activeSession, err := app.currentUser.GetActiveSessionID(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to query user's active session")
			return err
		}

		if len(sessions) > 0 {
			type chatDisplay struct {
				SessionID       string `yaml:"id"`
				CurrentlyActive bool   `yaml:"in-focus"`
				SessionState    string `yaml:"state"`
				Model           string `yaml:"model"`
				FirstRequest    string `yaml:"request"`
			}
			displayEntries := []chatDisplay{}

			// Go through the sessions
			for _, oneSession := range sessions {
				sessionID, err := oneSession.SessionID(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Session ID read failed")
					return err
				}
				sessionState, err := oneSession.SessionState(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Session state read failed")
					return err
				}
				setting, err := oneSession.Settings(app.ctxt)
				if err != nil {
					log.WithError(err).WithFields(logtags).Error("Session setting read failed")
					return err
				}
				displayEntry := chatDisplay{
					SessionID:    sessionID,
					SessionState: string(sessionState),
					Model:        setting.Model,
				}
				if activeSession != nil {
					if sessionID == *activeSession {
						displayEntry.CurrentlyActive = true
					}
				}
				displayEntries = append(displayEntries, displayEntry)
			}

			type toDisplay struct {
				AllSessions []chatDisplay `yaml:"sessions"`
			}
			display := toDisplay{AllSessions: displayEntries}

			// Display as YAML
			t, _ := yaml.Marshal(&display)

			fmt.Printf("%s\n", t)
		}

		return nil
	}
}

// ================================================================================

// standardChatActionCLIArgs standard cli arguments when working with a specific chat session
type standardChatActionCLIArgs struct {
	commonCLIArgs
	// SessionID the chat session ID
	SessionID string `validate:"required"`
}

/*
getCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *standardChatActionCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.GetCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "session-id",
			Usage:       "Target chat session ID",
			Aliases:     []string{"i"},
			EnvVars:     []string{"TARGET_SESSION_ID"},
			Destination: &c.SessionID,
			Required:    true,
		},
	}...)

	return cliFlags
}

var standardChatActionParams standardChatActionCLIArgs

/*
actionGetChatSessionDetails print details regarding the chat sessions

- session metadata

- all exchanges

	@param args *standardChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionGetChatSessionDetails(args *standardChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		// Get the associated chat manager
		chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
			return err
		}

		activeSession, err := app.currentUser.GetActiveSessionID(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to query user's active session")
			return err
		}

		session, err := chatManager.GetSession(app.ctxt, args.SessionID)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Could not fetch chat session '%s'", args.SessionID)
			return err
		}

		// Create the display
		type sessionDisplay struct {
			SessionID       string                            `yaml:"id"`
			CurrentlyActive bool                              `yaml:"in-focus"`
			SessionState    string                            `yaml:"state"`
			Settings        persistence.ChatSessionParameters `yaml:"settings"`
			Exchanges       []persistence.ChatExchange        `yaml:"exchanges"`
		}
		display := sessionDisplay{SessionID: args.SessionID}
		if activeSession != nil {
			if args.SessionID == *activeSession {
				display.CurrentlyActive = true
			}
		}

		state, err := session.SessionState(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Session state read failed")
			return err
		}
		display.SessionState = string(state)

		display.Settings, err = session.Settings(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Session setting read failed")
			return err
		}

		display.Exchanges, err = session.Exchanges(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Session exchanges read failed")
			return err
		}

		// Display as YAML
		t, _ := yaml.Marshal(&display)

		fmt.Printf("%s\n", t)

		return nil
	}
}

// ================================================================================

// updateChatSettingActionCLIArgs cli arguments to update the chat session setting
type updateChatSettingActionCLIArgs struct {
	commonCLIArgs
	// SessionID the chat session ID
	SessionID string `validate:"required"`
	// NewSettingFile new chat session setting file
	NewSettingFile string `validate:"required,file"`
}

/*
getCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *updateChatSettingActionCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.GetCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "session-id",
			Usage:       "Target chat session ID",
			Aliases:     []string{"i"},
			EnvVars:     []string{"TARGET_SESSION_ID"},
			Destination: &c.SessionID,
			Required:    true,
		},
		&cli.StringFlag{
			Name:        "settings-file",
			Usage:       "YAML file containing new chat session settings",
			Aliases:     []string{"f"},
			EnvVars:     []string{"CHAT_SESSION_SETTINGS_FILE"},
			Destination: &c.NewSettingFile,
			Required:    true,
		},
	}...)

	return cliFlags
}

var updateChatSettingParams updateChatSettingActionCLIArgs

/*
actionUpdateChatSessionSettings update the chat session settings

	@param args *updateChatSettingActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionUpdateChatSessionSettings(args *updateChatSettingActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
			return err
		}

		session, err := chatManager.GetSession(app.ctxt, args.SessionID)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Could not fetch chat session '%s'", args.SessionID)
			return err
		}

		// Get current settings
		currentSetting, err := session.Settings(app.ctxt)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Unable to read chat session '%s' settings", args.SessionID)
			return err
		}

		// Parse the new chat setting config
		settingContent, err := os.ReadFile(args.NewSettingFile)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Unable to read setting file '%s'", args.NewSettingFile)
			return err
		}
		var newSetting persistence.ChatSessionParameters
		if err := yaml.Unmarshal(settingContent, &newSetting); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Unable to parse setting file '%s'", args.NewSettingFile)
			return err
		}

		// Merge the new setting into the existing setting
		currentSetting.MergeWithNewSettings(newSetting)

		// Store the updated setting
		if err := session.ChangeSettings(app.ctxt, currentSetting); err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to apply new session setting")
			return err
		}

		return nil
	}
}

// ================================================================================

/*
actionChangeActiveChatSession change the active chat session for current active user

	@param args *standardChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionChangeActiveChatSession(args *standardChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		if err := app.currentUser.SetActiveSessionID(app.ctxt, args.SessionID); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Failed to set '%s' as active chat session", args.SessionID)
			return err
		}

		return nil
	}
}

// ================================================================================

/*
actionCloseChatSession close the chat session

	@param args *standardChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionCloseChatSession(args *standardChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
			return err
		}

		session, err := chatManager.GetSession(app.ctxt, args.SessionID)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Could not fetch chat session '%s'", args.SessionID)
			return err
		}

		if err := session.CloseSession(app.ctxt); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Unable to close chat session '%s'", args.SessionID)
			return err
		}
		return nil
	}
}

// ================================================================================

/*
actionDeleteChatSession delete chat session

	@param args *standardChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionDeleteChatSession(args *standardChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, err := args.initialSetup(validator.New(), "list-user")
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		logtags := app.GetLogTagsForContext(app.ctxt)

		if app.currentUser == nil {
			return fmt.Errorf("no active user selected")
		}

		// Get the associated chat manager
		chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
			return err
		}

		if err := chatManager.DeleteSession(app.ctxt, args.SessionID); err != nil {
			log.WithError(err).WithFields(logtags).Errorf("Unable to delete session '%s'", args.SessionID)
		}

		return nil
	}
}

// ================================================================================

/*
ActionAppendToChatSession append new exchange to active chat session

	@param args *commonCLIArgs - CLI arguments
	@return the CLI action
*/
func ActionAppendToChatSession(args *commonCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// TODO
		return nil
	}
}

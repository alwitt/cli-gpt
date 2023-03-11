package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/alwitt/cli-gpt/api"
	"github.com/alwitt/cli-gpt/persistence"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/manifoldco/promptui"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

// baseChatAppInitialization helper function to initialize chat related UX application
func baseChatAppInitialization(args cliArgContext) (
	*applicationContext, log.Fields, persistence.ChatSessionManager, error,
) {
	// Initialize application
	app, err := args.initialSetup(validator.New(), "list-user")
	if err != nil {
		log.WithError(err).Error("Failed to prepare new application")
		return nil, nil, nil, err
	}

	logtags := app.GetLogTagsForContext(app.ctxt)

	if app.currentUser == nil {
		return nil, nil, nil, fmt.Errorf("no active user selected")
	}

	// Get the associated chat manager
	chatManager, err := app.currentUser.ChatSessionManager(app.ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Could not define chat session manager")
		return nil, nil, nil, err
	}

	return app, logtags, chatManager, nil
}

// multilinePrompt prompt the user to input multi-line input
func multilinePrompt(ctxt context.Context) (string, error) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanLines)

	print("> ")
	// Start reading
	inputBuilder := strings.Builder{}
	for scanner.Scan() {
		oneLine := scanner.Text()
		if len(oneLine) == 0 {
			break
		}
		inputBuilder.WriteString(fmt.Sprintf("%s\n", oneLine))
	}

	return inputBuilder.String(), nil
}

// processOneChatExchange helper function to handle one chat exchange
func processOneChatExchange(
	app *applicationContext, session persistence.ChatSession, logtags log.Fields,
) error {
	prompt, err := multilinePrompt(app.ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to prompt for user request")
		return err
	}

	log.WithFields(logtags).Debugf("Your prompt:\n%s\n", prompt)

	promptBuilder, err := api.GetSimpleChatPromptBuilder()
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to define basic prompt builder")
		return err
	}

	client, err := api.GetClient(app.ctxt, app.currentUser, promptBuilder)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to define OpenAI API client")
		return err
	}

	chatHandler, err := api.DefineChatSessionHandler(app.ctxt, session, client)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to define chat handler")
		return err
	}

	log.WithFields(logtags).Debug("Defined chat handler")

	respChan := make(chan string)

	var reqErr error

	wg := sync.WaitGroup{}
	defer wg.Wait()
	wg.Add(1)
	go func() {
		defer wg.Done()
		if reqErr = chatHandler.SendRequest(app.ctxt, prompt, respChan); reqErr != nil {
			log.WithError(reqErr).WithFields(logtags).Error("Request-response failed")
		}
	}()

	terminate := false
	for !terminate {
		select {
		case <-app.ctxt.Done():
			terminate = true
		case msg, ok := <-respChan:
			if ok {
				print(msg)
				terminate = false
			} else {
				terminate = true
			}
		}
	}

	return reqErr
}

// ================================================================================

// startNewChatActionCLIArgs standard cli arguments when starting a new chat session
type startNewChatActionCLIArgs struct {
	commonCLIArgs
	// Model model to use
	Model string `validate:"required,oneof=turbo davinci curie babbage ada"`
	// SetAsActive whether to make this new chat the active chat session
	SetAsActive bool
}

func (c *startNewChatActionCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.GetCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringFlag{
			Name:        "model",
			Usage:       "Text generation model: [turbo davinci curie babbage ada]",
			Aliases:     []string{"m"},
			EnvVars:     []string{"TEXT_COMPLETION_MODEL"},
			Value:       "turbo",
			DefaultText: "turbo",
			Destination: &c.Model,
			Required:    false,
		},
		&cli.BoolFlag{
			Name:        "make-active",
			Usage:       "Make this new chat session the current active session for the user",
			Aliases:     []string{"a"},
			Value:       true,
			DefaultText: "true",
			Destination: &c.SetAsActive,
			Required:    false,
		},
	}...)

	return cliFlags
}

var startNewChatParams startNewChatActionCLIArgs

// Helper function to ask user for request parameters if settings file not provided
func askUserForChatRequestOptions(currentSetting persistence.ChatSessionParameters) (
	persistence.ChatSessionParameters, error,
) {
	newSetting := persistence.ChatSessionParameters{}

	var err error
	// Ask for model
	modelPrompt := promptui.Select{
		Label: "Select request model",
		Items: []string{"turbo", "davinci", "curie", "babbage", "ada"},
	}
	if _, newSetting.Model, err = modelPrompt.Run(); err != nil {
		return newSetting, err
	}

	// Ask for max token
	maxTokenPrompt := promptui.Prompt{
		Label:   "Max tokens per response",
		Default: fmt.Sprintf("%d", currentSetting.MaxTokens),
	}
	if maxTokenStr, err := maxTokenPrompt.Run(); err != nil {
		return newSetting, err
	} else if newSetting.MaxTokens, err = strconv.Atoi(maxTokenStr); err != nil {
		return newSetting, err
	}

	// Ask for temperature
	temperaturePrompt := promptui.Prompt{
		Label:   "Request temperature",
		Default: "0.8",
	}
	if currentSetting.Temperature != nil {
		temperaturePrompt.Default = fmt.Sprintf("%f", *currentSetting.Temperature)
	}
	if temperatureStr, err := temperaturePrompt.Run(); err != nil {
		return newSetting, err
	} else if temp64, err := strconv.ParseFloat(temperatureStr, 32); err != nil {
		return newSetting, err
	} else {
		temp32 := float32(temp64)
		newSetting.Temperature = &temp32
	}

	// Ask for TopP
	toppPrompt := promptui.Prompt{
		Label:   "Request TopP",
		Default: "0",
	}
	if currentSetting.TopP != nil {
		toppPrompt.Default = fmt.Sprintf("%f", *currentSetting.TopP)
	}
	if toppStr, err := toppPrompt.Run(); err != nil {
		return newSetting, err
	} else if temp64, err := strconv.ParseFloat(toppStr, 32); err != nil {
		return newSetting, err
	} else {
		temp32 := float32(temp64)
		newSetting.TopP = &temp32
	}

	// Ask for Presence Penalty
	presencePenaltyPrompt := promptui.Prompt{
		Label:   "Request Presence Penalty",
		Default: "",
	}
	if currentSetting.PresencePenalty != nil {
		presencePenaltyPrompt.Default = fmt.Sprintf("%f", *currentSetting.PresencePenalty)
	}
	if presencePenaltyStr, err := presencePenaltyPrompt.Run(); err != nil {
		return newSetting, err
	} else if len(presencePenaltyStr) > 0 {
		temp64, err := strconv.ParseFloat(presencePenaltyStr, 32)
		if err != nil {
			return newSetting, err
		}
		temp32 := float32(temp64)
		newSetting.PresencePenalty = &temp32
	} else {
		newSetting.PresencePenalty = nil
	}

	// Ask for Frequency Penalty
	frequencyPenaltyPrompt := promptui.Prompt{
		Label:   "Request Frequency Penalty",
		Default: "",
	}
	if currentSetting.FrequencyPenalty != nil {
		frequencyPenaltyPrompt.Default = fmt.Sprintf("%f", *currentSetting.FrequencyPenalty)
	}
	if frequencyPenaltyStr, err := frequencyPenaltyPrompt.Run(); err != nil {
		return newSetting, err
	} else if len(frequencyPenaltyStr) > 0 {
		temp64, err := strconv.ParseFloat(frequencyPenaltyStr, 32)
		if err != nil {
			return newSetting, err
		}
		temp32 := float32(temp64)
		newSetting.FrequencyPenalty = &temp32
	} else {
		newSetting.FrequencyPenalty = nil
	}

	return newSetting, nil
}

/*
actionStartNewChat start a new chat session

	@param args *startNewChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionStartNewChat(args *startNewChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		// Get chat session request parameters
		newSetting, err := askUserForChatRequestOptions(persistence.GetDefaultChatSessionParams("turbo"))
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to prompt user for parameters")
			return err
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

		if args.SetAsActive {
			if err := app.currentUser.SetActiveSessionID(app.ctxt, sessionID); err != nil {
				log.
					WithError(err).
					WithFields(logtags).
					Errorf("Failed to set '%s' as active chat session", sessionID)
				return err
			}
		}

		// Make the first exchange
		return processOneChatExchange(app, session, logtags)
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
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
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
				if firstExchange, err := oneSession.FirstExchange(app.ctxt); err == nil {
					displayEntry.FirstRequest = firstExchange.Request
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
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
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
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
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

		// Get chat session request parameters
		newSetting, err := askUserForChatRequestOptions(currentSetting)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to prompt user for parameters")
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
		app, logtags, _, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

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
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
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

// deleteChatSessionsCLIArgs cli arguments to delete chat sessions
type deleteChatSessionsCLIArgs struct {
	commonCLIArgs
	// SessionIDs the chat session IDs to delete
	SessionIDs cli.StringSlice
	// DeleteAll whether to delete all sessions
	DeleteAll bool
}

/*
getCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *deleteChatSessionsCLIArgs) getCLIFlags() []cli.Flag {
	// Get the common CLI flags
	cliFlags := c.GetCommonCLIFlags()

	// Attach CLI arguments needed for this action
	cliFlags = append(cliFlags, []cli.Flag{
		&cli.StringSliceFlag{
			Name:        "session-id",
			Usage:       "Target chat session ID",
			Aliases:     []string{"i"},
			EnvVars:     []string{"TARGET_SESSION_ID"},
			Destination: &c.SessionIDs,
			Required:    false,
		},
		&cli.BoolFlag{
			Name:        "delete-all",
			Usage:       "Whether to delete all sessions",
			Aliases:     []string{"A"},
			Destination: &c.DeleteAll,
			Required:    false,
		},
	}...)

	return cliFlags
}

var deleteChatSessionsParams deleteChatSessionsCLIArgs

/*
actionDeleteChatSession delete chat session

	@param args *deleteChatSessionsCLIArgs - CLI arguments
	@return the CLI action
*/
func actionDeleteChatSession(args *deleteChatSessionsCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		if len(args.SessionIDs.Value()) < 1 && !args.DeleteAll {
			return fmt.Errorf("must provide at least one ID, or must delete ALL session")
		}

		// Initialize application
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		if args.DeleteAll {
			if err := chatManager.DeleteAllSessions(app.ctxt); err != nil {
				log.WithError(err).WithFields(logtags).Error("Failed to delete all sessions")
				return err
			}
		} else {
			sessionIDs := args.SessionIDs.Value()
			if err := chatManager.DeleteMultipleSessions(app.ctxt, sessionIDs); err != nil {
				t, _ := json.Marshal(&sessionIDs)
				log.
					WithError(err).
					WithFields(logtags).
					Errorf("Unable to delete session %s", t)
			}
		}
		return nil
	}
}

// ================================================================================

/*
actionDeleteLatestExchange delete latest chat session exchange

	@param args *standardChatActionCLIArgs - CLI arguments
	@return the CLI action
*/
func actionDeleteLatestExchange(args *standardChatActionCLIArgs) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		// Initialize application
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
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

		if err := session.DeleteLatestExchange(app.ctxt); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Unable to delete chat session '%s' latest exchange", args.SessionID)
			return err
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
		// Initialize application
		app, logtags, chatManager, err := baseChatAppInitialization(args)
		if err != nil {
			log.WithError(err).Error("Failed to prepare new application")
			return err
		}

		session, err := chatManager.CurrentActiveSession(app.ctxt)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Error("Could not fetch active chat session")
			return err
		}

		return processOneChatExchange(app, session, logtags)
	}
}

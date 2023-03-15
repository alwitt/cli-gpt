package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"path/filepath"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/alwitt/goutils"
	"github.com/apex/log"
	apexJSON "github.com/apex/log/handlers/json"
	"github.com/go-playground/validator/v10"
	"github.com/urfave/cli/v2"
	gormLogger "gorm.io/gorm/logger"
)

// loggingArgs cli arguments related to logging
type loggingArgs struct {
	// JSONLog whether to produce JSON formated logs
	JSONLog bool
	// LogLevel set the logging level
	LogLevel string `validate:"required,oneof=debug info warn error"`
}

// configFileArgs cli arguments related to config files needed by the application
type configFileArgs struct {
	// UserContext JSON config file containing the current active user context
	UserContext string `validate:"required"`
	// SqliteDB sqlite DB file for persistence
	SqliteDB string `validate:"required"`
}

/*
touchFile verify file path is exists, if not create it.

	@param path string - file path
	@param createFile bool - whether to create the file if missing, or just create the DIR path
*/
func touchFile(path string, createFile bool) (string, error) {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absPath); errors.Is(err, os.ErrNotExist) {
		// Ensure the DIR path exists
		if err := os.MkdirAll(filepath.Dir(absPath), 0770); err != nil {
			return "", err
		}
		// Create file if requested
		if createFile {
			if _, err := os.Create(absPath); err != nil {
				return "", nil
			}
		}
	}
	return absPath, nil
}

// Prepare the config file by ensuring that the DIR and file exists
func (c *configFileArgs) initialize() error {
	fileName, err := touchFile(c.SqliteDB, false)
	if err != nil {
		return err
	}
	c.SqliteDB = fileName
	fileName, err = touchFile(c.UserContext, true)
	if err != nil {
		return err
	}
	c.UserContext = fileName
	return nil
}

// commonCLIArgs cli arguments needed for operating against all APIs
type commonCLIArgs struct {
	// Logging logging related arguments
	Logging loggingArgs `validate:"required,dive"`
	// Config config file related arguments
	Config configFileArgs `validate:"required,dive"`
}

/*
GetCommonCLIFlags fetch the list of CLI arguments

	@return the list of CLI arguments
*/
func (c *commonCLIArgs) GetCommonCLIFlags() []cli.Flag {
	usr, err := user.Current()
	if err != nil {
		log.WithError(err).Fatal("Unable to query user of calling user")
		return nil
	}
	userContextFile := filepath.Join(usr.HomeDir, ".config", "cli-gpt", "user_context.json")
	sqliteDB := filepath.Join(usr.HomeDir, ".config", "cli-gpt", "persistence.db")

	return []cli.Flag{
		// LOGGING
		&cli.BoolFlag{
			Name:        "json-log",
			Usage:       "Whether to log in JSON format",
			Aliases:     []string{"j"},
			EnvVars:     []string{"LOG_AS_JSON"},
			Value:       false,
			DefaultText: "false",
			Destination: &c.Logging.JSONLog,
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "log-level",
			Usage:       "Logging level: [debug info warn error]",
			Aliases:     []string{"l"},
			EnvVars:     []string{"LOG_LEVEL"},
			Value:       "error",
			DefaultText: "error",
			Destination: &c.Logging.LogLevel,
			Required:    false,
		},
		// Application Config
		&cli.StringFlag{
			Name:        "user-context-file",
			Usage:       "JSON config file containing the current active user context",
			Aliases:     []string{"ucf"},
			EnvVars:     []string{"USER_CONTEXT_FILE"},
			Value:       userContextFile,
			DefaultText: userContextFile,
			Destination: &c.Config.UserContext,
			Required:    false,
		},
		&cli.StringFlag{
			Name:        "sqlite-persistence-db",
			Usage:       "Sqlite DB file for persistence",
			Aliases:     []string{"spd"},
			EnvVars:     []string{"SQLITE_PERSISTENCE_DATABASE"},
			Value:       sqliteDB,
			DefaultText: sqliteDB,
			Destination: &c.Config.SqliteDB,
			Required:    false,
		},
	}
}

// CommonParams basic CLI arguments
var CommonParams commonCLIArgs

/*
initialSetup perform basic application setup

	@param validate *validator.Validate - validation engine
	@param appInstance string - application instance name
	@return new application context
*/
func (c *commonCLIArgs) initialSetup(
	validate *validator.Validate, appInstance string,
) (*applicationContext, error) {
	if err := validate.Struct(c); err != nil {
		return nil, err
	}
	if c.Logging.JSONLog {
		log.SetHandler(apexJSON.New(os.Stderr))
	}
	var sqlLogLevel gormLogger.LogLevel
	switch c.Logging.LogLevel {
	case "debug":
		sqlLogLevel = gormLogger.Info
		log.SetLevel(log.DebugLevel)
	case "info":
		sqlLogLevel = gormLogger.Warn
		log.SetLevel(log.InfoLevel)
	case "warn":
		sqlLogLevel = gormLogger.Error
		log.SetLevel(log.WarnLevel)
	case "error":
		sqlLogLevel = gormLogger.Silent
		log.SetLevel(log.ErrorLevel)
	default:
		sqlLogLevel = gormLogger.Silent
		log.SetLevel(log.ErrorLevel)
	}
	{
		tmp, _ := json.Marshal(c)
		log.Debugf("Starting common params %s", tmp)
	}

	// Prepare application context
	newContext := defineApplicationContext(context.Background(), c.Config, appInstance)
	if err := newContext.initialize(sqlLogLevel); err != nil {
		log.WithError(err).Error("Failed to initialize application context")
		return nil, err
	}

	return newContext, nil
}

// cliArgContext standard CLI argument context object
type cliArgContext interface {
	/*
		initialSetup perform basic application setup

			@param validate *validator.Validate - validation engine
			@param appInstance string - application instance name
			@return new application context
	*/
	initialSetup(validate *validator.Validate, appInstance string) (*applicationContext, error)
}

// ================================================================================

// applicationContext application context
type applicationContext struct {
	goutils.Component
	ctxt        context.Context
	config      configFileArgs
	currentUser persistence.User
	userManager persistence.UserManager
}

// userContext the contents of the user context file
type userContext struct {
	CurrentUserID string `json:"current_user_id" validate:"required"`
}

/*
defineApplicationContext define new application context

	@param ctxt context.Context - application context
	@param config configFileArgs - application configuration related parameters
	@param instance string - application instance description
	@return new application context
*/
func defineApplicationContext(
	ctxt context.Context, config configFileArgs, instance string,
) *applicationContext {
	logTags := log.Fields{
		"module": "cmd", "component": "main", "instance": instance,
	}
	return &applicationContext{
		Component: goutils.Component{
			LogTags:         logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		}, ctxt: ctxt, config: config, currentUser: nil, userManager: nil,
	}
}

// Initialize application context
func (c *applicationContext) initialize(sqlLogLevel gormLogger.LogLevel) error {
	logtags := c.GetLogTagsForContext(c.ctxt)

	if err := c.config.initialize(); err != nil {
		log.WithError(err).WithFields(logtags).Error("Config file setup failed")
		return err
	}

	// Open sqlite DB
	log.WithFields(logtags).Debugf("Opening sqlite persistence DB '%s'", c.config.SqliteDB)
	manager, err := persistence.GetSQLUserManager(
		persistence.GetSqliteDialector(c.config.SqliteDB), sqlLogLevel,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			Errorf("Unable to open sqlite persistence DB '%s'", c.config.SqliteDB)
		return err
	}
	log.WithFields(logtags).Debugf("Define user manager using persistence DB '%s'", c.config.SqliteDB)

	c.userManager = manager

	// Process user context file, if it is filled
	contextContent, err := os.ReadFile(c.config.UserContext)
	if err != nil {
		log.WithError(err).WithFields(logtags).Errorf("Unable to read user context file")
		return err
	}
	if len(contextContent) > 0 {
		var contextParam userContext
		if err := json.Unmarshal(contextContent, &contextParam); err != nil {
			log.WithError(err).WithFields(logtags).Errorf("Unable to parse user context file")
			return err
		}
		userEntry, err := c.userManager.GetUser(c.ctxt, contextParam.CurrentUserID)
		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Unable to fetch user '%s'", contextParam.CurrentUserID)
			return err
		}
		c.currentUser = userEntry
	}

	return nil
}

// Record current application context
func (c *applicationContext) record() error {
	logtags := c.GetLogTagsForContext(c.ctxt)

	contextFile, err := os.Create(c.config.UserContext)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			Errorf("Failed to open user context file '%s'", c.config.UserContext)
		return err
	}
	defer func() {
		if err := contextFile.Close(); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("User context file '%s' close failure", c.config.UserContext)
		}
	}()

	// Write user context back
	if c.currentUser != nil {
		userID, err := c.currentUser.GetID(c.ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Could not read current user ID")
			return nil
		}
		content := userContext{CurrentUserID: userID}
		t, _ := json.MarshalIndent(&content, "", "  ")
		if _, err := contextFile.Write(t); err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				Errorf("Failed to write user context file '%s'", c.config.UserContext)
			return err
		}
	}

	return nil
}

// ================================================================================

/*
GenerateGetSubcommands generate list of subcommands for "get"

	@return the list of CLI subcommands
*/
func GenerateGetSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "users",
			Aliases:     []string{"user"},
			Usage:       "List registered users",
			Description: "List registered users",
			Flags:       CommonParams.GetCommonCLIFlags(),
			Action:      actionListUsers(&CommonParams),
		},
		{
			Name:        "chats",
			Aliases:     []string{"chat"},
			Usage:       "List chat sessions",
			Description: "List chat sessions associated with the currently active user",
			Flags:       CommonParams.GetCommonCLIFlags(),
			Action:      actionListChatSession(&CommonParams),
		},
	}
}

/*
GenerateDescribeSubcommands generate list of subcommands for "describe"

	@return the list of CLI subcommands
*/
func GenerateDescribeSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "chats",
			Aliases:     []string{"chat"},
			Usage:       "Print details of a chat session",
			Description: "Print details of a chat session",
			Flags:       describeChatActionParams.getCLIFlags(),
			Action:      actionGetChatSessionDetails(&describeChatActionParams),
		},
	}
}

/*
GenerateCreateSubcommands generate list of subcommands for "create"

	@return the list of CLI subcommands
*/
func GenerateCreateSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "user",
			Aliases:     []string{"users"},
			Usage:       "Register new user",
			Description: "Register new user",
			Flags:       CommonParams.GetCommonCLIFlags(),
			Action:      actionCreateUser(&CommonParams),
		},
		{
			Name:        "chat",
			Aliases:     []string{"chats"},
			Usage:       "Start new chat session",
			Description: "Start new chat session for currently active user",
			Flags:       startNewChatParams.getCLIFlags(),
			Action:      actionStartNewChat(&startNewChatParams),
		},
	}
}

/*
GenerateUpdateSubcommands generate list of subcommands for "update"

	@return the list of CLI subcommands
*/
func GenerateUpdateSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "user",
			Aliases:     []string{"users"},
			Usage:       "Update user settings",
			Description: "Update user settings",
			Flags:       specifyUserParams.getCLIFlags(),
			Action:      actionUpdateUser(&specifyUserParams),
		},
		{
			Name:        "chat",
			Aliases:     []string{"chats"},
			Usage:       "Update chat session request settings",
			Description: "Update chat session request settings",
			Flags:       standardChatActionParams.getCLIFlags(),
			Action:      actionUpdateChatSessionSettings(&standardChatActionParams),
		},
	}
}

/*
GenerateDeleteSubcommands generate list of subcommands for "delete"

	@return the list of CLI subcommands
*/
func GenerateDeleteSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "user",
			Aliases:     []string{"users"},
			Usage:       "Delete currently active user",
			Description: "Delete currently active user. This will also delete this user's data.",
			Flags:       specifyUserParams.getCLIFlags(),
			Action:      actionDeleteUser(&specifyUserParams),
		},
		{
			Name:        "chat",
			Aliases:     []string{"chats"},
			Usage:       "Delete chat session",
			Description: "Delete chat session",
			Flags:       deleteChatSessionsParams.getCLIFlags(),
			Action:      actionDeleteChatSession(&deleteChatSessionsParams),
		},
		{
			Name:        "latest-exchange",
			Aliases:     []string{"le"},
			Usage:       "Delete latest exchange",
			Description: "Delete the latest exchange of a chat session",
			Flags:       standardChatActionParams.getCLIFlags(),
			Action:      actionDeleteLatestExchange(&standardChatActionParams),
		},
	}
}

/*
GenerateContextSubcommands generate list of subcommands for "delete"

	@return the list of CLI subcommands
*/
func GenerateContextSubcommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:        "select-user",
			Usage:       "Change active user",
			Description: "Change the currently active user",
			Flags:       specifyUserParams.getCLIFlags(),
			Action:      actionChangeActiveUser(&specifyUserParams),
		},
		{
			Name:        "select-chat",
			Usage:       "Change active chat session",
			Description: "Change active chat session for current user",
			Flags:       standardChatActionParams.getCLIFlags(),
			Action:      actionChangeActiveChatSession(&standardChatActionParams),
		},
		{
			Name:        "close-chat",
			Usage:       "Close chat session",
			Description: "Close chat session. User can not append to session after closing",
			Flags:       standardChatActionParams.getCLIFlags(),
			Action:      actionCloseChatSession(&standardChatActionParams),
		},
	}
}

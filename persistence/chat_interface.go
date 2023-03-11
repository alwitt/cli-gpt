package persistence

import (
	"context"
	"time"
)

// ChatSessionState chat session state variable
type ChatSessionState string

const (
	// DefaultChatMaxResponseTokens default max token count for chat session response
	DefaultChatMaxResponseTokens = 2048
	// DefaultChatRequestTemperature default chat request temperature
	DefaultChatRequestTemperature = float32(1.0)
	// DefaultChatRequestTopP default chat request TopP
	DefaultChatRequestTopP = float32(0)

	// ChatSessionStateOpen ENUM for chat session state "OPEN"
	ChatSessionStateOpen ChatSessionState = "session-open"
	// ChatSessionStateClose ENUM for chat session state "CLOSE"
	ChatSessionStateClose ChatSessionState = "session-close"
)

/*
ChatSessionParameters common API request parameters used for one chat session

See https://platform.openai.com/docs/api-reference/completions/create for explanations
*/
type ChatSessionParameters struct {
	Model            string   `yaml:"model" json:"model" validate:"required,oneof=turbo davinci curie babbage ada"`
	Suffix           *string  `yaml:"suffix,omitempty" json:"suffix,omitempty"`
	MaxTokens        int      `yaml:"max_tokens" json:"max_tokens" validate:"required,gte=10,lte=4096"`
	Temperature      *float32 `yaml:"temperature,omitempty" json:"temperature,omitempty" validate:"omitempty,gte=0,lte=2"`
	TopP             *float32 `yaml:"top_p,omitempty" json:"top_p,omitempty" validate:"omitempty,gte=0,lte=1"`
	Stop             []string `yaml:"stop,omitempty" json:"stop,omitempty" validate:"omitempty,lte=4"`
	PresencePenalty  *float32 `yaml:"presence_penalty,omitempty" json:"presence_penalty,omitempty" validate:"omitempty,gte=-2,lte=2"`
	FrequencyPenalty *float32 `yaml:"frequency_penalty,omitempty" json:"frequency_penalty,omitempty" validate:"omitempty,gte=-2,lte=2"`
}

/*
GetDefaultChatSessionParams generate default chat session request params

	@param model string - chat session model
	@return default chat session parameters
*/
func GetDefaultChatSessionParams(model string) ChatSessionParameters {
	defaultTemp := DefaultChatRequestTemperature
	defaultTopP := DefaultChatRequestTopP
	return ChatSessionParameters{
		Model:       model,
		MaxTokens:   DefaultChatMaxResponseTokens,
		Temperature: &defaultTemp,
		TopP:        &defaultTopP,
	}
}

/*
MergeWithNewSettings merge the contents of the new setting into current setting

Only fields in the new setting which are not nil will be merged in

	@param newSetting ChatSessionParameters - new setting
*/
func (s *ChatSessionParameters) MergeWithNewSettings(newSetting ChatSessionParameters) {
	s.Model = newSetting.Model
	if newSetting.Suffix != nil {
		s.Suffix = newSetting.Suffix
	}
	s.MaxTokens = newSetting.MaxTokens
	if newSetting.Temperature != nil {
		s.Temperature = newSetting.Temperature
	}
	if newSetting.TopP != nil {
		s.TopP = newSetting.TopP
	}
	if len(newSetting.Stop) > 0 {
		s.Stop = newSetting.Stop
	}
	if newSetting.PresencePenalty != nil {
		s.PresencePenalty = newSetting.PresencePenalty
	}
	if newSetting.FrequencyPenalty != nil {
		s.FrequencyPenalty = newSetting.FrequencyPenalty
	}
}

/*
ChatExchange defines one exchange during a chat session.

An exchange is defined as a request and its associated response
*/
type ChatExchange struct {
	RequestTimestamp  time.Time `yaml:"request_ts" json:"request_ts" validate:"required"`
	Request           string    `yaml:"request" json:"request" validate:"required"`
	ResponseTimestamp time.Time `yaml:"response_ts" json:"response_ts" validate:"required"`
	Response          string    `yaml:"response" json:"response" validate:"required"`
}

/*
ChatSession define a chat session with a text completion model.

This records the requests and responses between the user and the model.
*/
type ChatSession interface {
	/*
		SessionID this chat session ID

			@param ctxt context.Context - query context
			@return session ID
	*/
	SessionID(ctxt context.Context) (string, error)

	/*
		SessionState session's current state

			@param ctxt context.Context - query context
			@return current session state
	*/
	SessionState(ctxt context.Context) (ChatSessionState, error)

	/*
		CloseSession close this chat session

			@param ctxt context.Context - query context
	*/
	CloseSession(ctxt context.Context) error

	/*
		User query the associated user for this chat session

			@param ctxt context.Context - query context
			@return the associated User
	*/
	User(ctxt context.Context) (User, error)

	/*
		Settings returns the current session wide API request parameters

			@param ctxt context.Context - query context
			@return session wide parameters
	*/
	Settings(ctxt context.Context) (ChatSessionParameters, error)

	/*
		ChangeSettings update the session wide API request parameters

			@param ctxt context.Context - query context
			@param newSettings ChatSessionParameters - new session wide API request parameters
	*/
	ChangeSettings(ctxt context.Context, newSettings ChatSessionParameters) error

	/*
		RecordOneExchange record a single exchange.

		An exchange is defined as a request and its associated response

			@param ctxt context.Context - query context
			@param exchange ChatExchange - the exchange
	*/
	RecordOneExchange(ctxt context.Context, exchange ChatExchange) error

	/*
		FirstExchange get the first session exchange

			@param ctxt context.Context - query context
			@return chat exchange
	*/
	FirstExchange(ctxt context.Context) (ChatExchange, error)

	/*
		Exchanges fetch the list of exchanges recorded in this session.

		The exchanges are sorted by chronological order.

			@param ctxt context.Context - query context
			@return list of exchanges in chronological order
	*/
	Exchanges(ctxt context.Context) ([]ChatExchange, error)

	/*
		Refresh helper function to sync the handler with what is stored in persistence

			@param ctxt context.Context - query context
	*/
	Refresh(ctxt context.Context) error

	/*
		DeleteLatestExchange delete the latest exchange in the session

			@param ctxt context.Context - query context
	*/
	DeleteLatestExchange(ctxt context.Context) error
}

/*
ChatSessionManager chat session management client
*/
type ChatSessionManager interface {
	/*
		NewSession define a new chat session

			@param ctxt context.Context - query context
			@param model stirng - OpenAI model name
			@return	new chat session
	*/
	NewSession(ctxt context.Context, model string) (ChatSession, error)

	/*
		ListSessions list all sessions

			@param ctxt context.Context - query context
			@return all known sessions
	*/
	ListSessions(ctxt context.Context) ([]ChatSession, error)

	/*
		GetSession fetch a session

			@param ctxt context.Context - query context
			@param sessionID string - session ID
			@return session entry
	*/
	GetSession(ctxt context.Context, sessionID string) (ChatSession, error)

	/*
	   CurrentActiveSession get the current active chat session for the associated user

	   	@param ctxt context.Context - query context
	   	@return session entry
	*/
	CurrentActiveSession(ctxt context.Context) (ChatSession, error)

	/*
	   SetActiveSession set the current active chat session for the associated user

	   	@param ctxt context.Context - query context
	   	@param session ChatSession - the chat session
	*/
	SetActiveSession(ctxt context.Context, session ChatSession) error

	/*
		DeleteSession delete a session

			@param ctxt context.Context - query context
			@param sessionID string - session ID
	*/
	DeleteSession(ctxt context.Context, sessionID string) error

	/*
		DeleteMultipleSessions delete multiple sessions

			@param ctxt context.Context - query context
			@param sessionIDs []string - session IDs
	*/
	DeleteMultipleSessions(ctxt context.Context, sessionIDs []string) error
}

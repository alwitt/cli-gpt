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
	Suffix           *string  `json:"suffix,omitempty"`
	MaxTokens        int      `json:"max_tokens"`
	Temperature      *float32 `json:"temperature,omitempty" validate:"omitempty,gte=0,lte=2"`
	TopP             *float32 `json:"top_p,omitempty" validate:"omitempty,gte=0,lte=1"`
	Stop             []string `json:"stop,omitempty" validate:"omitempty,lte=4"`
	PresencePenalty  *float32 `json:"presence_penalty,omitempty" validate:"omitempty,gte=-2,lte=2"`
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty" validate:"omitempty,gte=-2,lte=2"`
}

/*
ChatExchange defines one exchange during a chat session.

An exchange is defined as a request and its associated response
*/
type ChatExchange struct {
	RequestTimestamp  time.Time `validate:"required"`
	Request           string    `validate:"required"`
	ResponseTimestamp time.Time `validate:"required"`
	Response          string    `validate:"required"`
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
		CurrentModel query for currently selected text model

			@param ctxt context.Context - query context
			@return the current model name
	*/
	CurrentModel(ctxt context.Context) (string, error)

	/*
		ChangeModel change to model for the session

			@param ctxt context.Context - query context
			@param newModel string - the name of the new model
	*/
	ChangeModel(ctxt context.Context, newModel string) error

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
		DeleteSession delete a session

			@param ctxt context.Context - query context
			@param sessionID string - session ID
	*/
	DeleteSession(ctxt context.Context, sessionID string) error
}

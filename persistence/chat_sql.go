package persistence

import (
	"context"
	"time"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

// sqlChatSessionEntry SQL table representing a chat session
type sqlChatSessionEntry struct {
	// ID chat session entry ID
	ID string `gorm:"primaryKey"`
	// State chat session
	State ChatSessionState `gorm:"not null;type:varchar(64)"`
	// UserID ID of the user using this chat session
	UserID string       `gorm:"not null;index:chat_session_user_id"`
	User   sqlUserEntry `gorm:"constraint:OnDelete:CASCADE;foreignKey:UserID"`
	// Model the OpenAI model used
	Model string `gorm:"not null"`
	// CommonSettings common session parameters
	CommonSettings ChatSessionParameters  `gorm:"not null;type:text;serializer:json"`
	Exchanges      []sqlChatExchangeEntry `gorm:"foreignKey:SessionID"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// TableName hard code table name
func (sqlChatSessionEntry) TableName() string {
	return "chat_sessions"
}

/*
sqlChatExchangeEntry SQL table representing one chat session exchange

An exchange is one request and one response
*/
type sqlChatExchangeEntry struct {
	// ID chat exchange entry ID
	ID string `gorm:"primaryKey"`
	// SessionID ID of the session this exchange is attached to
	SessionID string              `gorm:"not null;index:chat_exchange_session_id"`
	Session   sqlChatSessionEntry `gorm:"constraint:OnDelete:CASCADE;foreignKey:SessionID"`
	// Request the user request
	Request string `gorm:"not null;type:text"`
	// RequestTimestamp when the request was made
	RequestTimestamp time.Time `gorm:"not null"`
	// Response the model response
	Response string `gorm:"not null;type:text"`
	// ResponseTimestamp when the response was received
	ResponseTimestamp time.Time `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// TableName hard code table name
func (sqlChatExchangeEntry) TableName() string {
	return "chat_session_exchanges"
}

// sqlChatSessionHandle wrapper object for working with the "chat_sessions" table
type sqlChatSessionHandle struct {
	goutils.Component
	driver    *sqlChatPersistance
	validator *validator.Validate
	sqlChatSessionEntry
}

/*
SessionID this chat session ID

	@param ctxt context.Context - query context
	@return session ID
*/
func (h *sqlChatSessionHandle) SessionID(ctxt context.Context) (string, error) {
	return h.ID, nil
}

/*
SessionState session's current state

	@param ctxt context.Context - query context
	@return current session state
*/
func (h *sqlChatSessionHandle) SessionState(ctxt context.Context) (ChatSessionState, error) {
	return h.State, nil
}

/*
CloseSession close this chat session

	@param ctxt context.Context - query context
*/
func (h *sqlChatSessionHandle) CloseSession(ctxt context.Context) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.
			Model(&h.sqlChatSessionEntry).
			Updates(&sqlChatSessionEntry{State: ChatSessionStateClose}).
			First(&h.sqlChatSessionEntry)
		if tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to update session state to '%s'", ChatSessionStateClose)
			return tmp.Error
		}
		return nil
	})
}

/*
User query the associated user for this chat session

	@param ctxt context.Context - query context
	@return the associated User
*/
func (h *sqlChatSessionHandle) User(ctxt context.Context) (User, error) {
	return h.driver.user, nil
}

/*
CurrentModel query for currently selected text model

	@param ctxt context.Context - query context
	@return the current model name
*/
func (h *sqlChatSessionHandle) CurrentModel(ctxt context.Context) (string, error) {
	return h.Model, nil
}

/*
ChangeModel change to model for the session

	@param ctxt context.Context - query context
	@param newModel string - the name of the new model
*/
func (h *sqlChatSessionHandle) ChangeModel(ctxt context.Context, newModel string) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.
			Model(&h.sqlChatSessionEntry).
			Updates(&sqlChatSessionEntry{Model: newModel}).
			First(&h.sqlChatSessionEntry)
		if tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to update session model to '%s'", newModel)
			return tmp.Error
		}
		return nil
	})
}

/*
Settings returns the current session wide API request parameters

	@param ctxt context.Context - query context
	@return session wide parameters
*/
func (h *sqlChatSessionHandle) Settings(ctxt context.Context) (ChatSessionParameters, error) {
	return h.CommonSettings, nil
}

/*
ChangeSettings update the session wide API request parameters

	@param ctxt context.Context - query context
	@param newSettings ChatSessionParameters - new session wide API request parameters
*/
func (h *sqlChatSessionHandle) ChangeSettings(
	ctxt context.Context, newSettings ChatSessionParameters,
) error {
	logtags := h.GetLogTagsForContext(ctxt)
	if err := h.validator.Struct(&newSettings); err != nil {
		log.WithError(err).WithFields(logtags).Error("New setting not valid")
		return err
	}
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.
			Model(&h.sqlChatSessionEntry).
			Updates(&sqlChatSessionEntry{CommonSettings: newSettings}).
			First(&h.sqlChatSessionEntry)
		if tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Error("Failed to update session common settings")
			return tmp.Error
		}
		return nil
	})
}

/*
RecordOneExchange record a single exchange.

An exchange is defined as a request and its associated response

	@param ctxt context.Context - query context
	@param exchange ChatExchange - the exchange
*/
func (h *sqlChatSessionHandle) RecordOneExchange(ctxt context.Context, exchange ChatExchange) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		exchangeID := ulid.Make().String()

		log.WithFields(logtags).Debugf("Define new chat exchange '%s'", exchangeID)

		// Define new exchange entry
		newEntry := sqlChatExchangeEntry{
			ID:                exchangeID,
			SessionID:         h.ID,
			Request:           exchange.Request,
			RequestTimestamp:  exchange.RequestTimestamp,
			Response:          exchange.Response,
			ResponseTimestamp: exchange.ResponseTimestamp,
		}
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to define new entry for chat exchange '%s'", exchangeID)
			return tmp.Error
		}

		log.WithFields(logtags).Debugf("Defined new chat exchange '%s'", exchangeID)

		return nil
	})
}

/*
FirstExchange get the first session exchange

	@param ctxt context.Context - query context
	@return chat exchange
*/
func (h *sqlChatSessionHandle) FirstExchange(ctxt context.Context) (ChatExchange, error) {
	logtags := h.GetLogTagsForContext(ctxt)
	var result ChatExchange
	return result, h.driver.db.Transaction(func(tx *gorm.DB) error {
		var entry sqlChatExchangeEntry

		if tmp := tx.
			Where(&sqlChatExchangeEntry{SessionID: h.ID}).
			Order("request_timestamp").
			First(&entry); tmp.Error != nil {
			log.WithError(tmp.Error).WithFields(logtags).Error("Failed to get first session exchange")
			return tmp.Error
		}

		result = ChatExchange{
			RequestTimestamp:  entry.RequestTimestamp,
			Request:           entry.Request,
			ResponseTimestamp: entry.ResponseTimestamp,
			Response:          entry.Response,
		}
		return nil
	})
}

/*
Exchanges fetch the list of exchanges recorded in this session.

The exchanges are sorted by chronological order.

	@param ctxt context.Context - query context
	@return list of exchanges in chronological order
*/
func (h *sqlChatSessionHandle) Exchanges(ctxt context.Context) ([]ChatExchange, error) {
	logtags := h.GetLogTagsForContext(ctxt)
	result := []ChatExchange{}
	return result, h.driver.db.Transaction(func(tx *gorm.DB) error {
		var entries []sqlChatExchangeEntry

		if tmp := tx.
			Where(&sqlChatExchangeEntry{SessionID: h.ID}).
			Order("request_timestamp").
			Find(&entries); tmp.Error != nil {
			log.WithError(tmp.Error).WithFields(logtags).Error("Failed to get session exchanges")
			return tmp.Error
		}

		for _, entry := range entries {
			result = append(result, ChatExchange{
				RequestTimestamp:  entry.RequestTimestamp,
				Request:           entry.Request,
				ResponseTimestamp: entry.ResponseTimestamp,
				Response:          entry.Response,
			})
		}
		return nil
	})
}

/*
Refresh helper function to sync the handler with what is stored in persistence

	@param ctxt context.Context - query context
*/
func (h *sqlChatSessionHandle) Refresh(ctxt context.Context) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.
			Where(&sqlChatSessionEntry{ID: h.ID}).
			First(&h.sqlChatSessionEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to refresh chat session '%s' info", h.ID)
			return tmp.Error
		}
		return nil
	})
}

// ============================================================================================
// SQL Chat Session Manager implementation

// defineSessionHandle helper function for defining new session handle object
func (c *sqlChatPersistance) defineSessionHandle(ctxt context.Context, sessionEntry sqlChatSessionEntry) sqlChatSessionHandle {
	logtags := c.GetLogTagsForContext(ctxt)
	logtags["session"] = sessionEntry.ID
	return sqlChatSessionHandle{
		Component: goutils.Component{
			LogTags:         logtags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		},
		driver:              c,
		validator:           validator.New(),
		sqlChatSessionEntry: sessionEntry,
	}
}

/*
NewSession define a new chat session

	@param ctxt context.Context - query context
	@param model stirng - OpenAI model name
	@return	new chat session
*/
func (c *sqlChatPersistance) NewSession(ctxt context.Context, model string) (ChatSession, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	var result sqlChatSessionHandle
	return &result, c.db.Transaction(func(tx *gorm.DB) error {
		sessionID := ulid.Make().String()

		log.WithFields(logtags).Debugf("Defining new chat session '%s'", sessionID)

		userID, err := c.user.GetID(ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to get associated user ID")
			return err
		}

		// Define a new session entry
		newEntry := sqlChatSessionEntry{
			ID:             sessionID,
			State:          ChatSessionStateOpen,
			UserID:         userID,
			Model:          model,
			CommonSettings: ChatSessionParameters{MaxTokens: DefaultChatMaxResponseTokens},
		}
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to define new entry for session '%s'", sessionID)
			return tmp.Error
		}

		log.WithFields(logtags).Debugf("Defined new chat session '%s'", sessionID)

		// Prepare wrapper object
		result = c.defineSessionHandle(ctxt, newEntry)
		return nil
	})
}

/*
ListSessions list all sessions

	@param ctxt context.Context - query context
	@return all known sessions
*/
func (c *sqlChatPersistance) ListSessions(ctxt context.Context) ([]ChatSession, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	result := []ChatSession{}
	return result, c.db.Transaction(func(tx *gorm.DB) error {
		var dbEntries []sqlChatSessionEntry

		userID, err := c.user.GetID(ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to get associated user ID")
			return err
		}

		if tmp := tx.Where(&sqlChatSessionEntry{UserID: userID}).Find(&dbEntries); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Error("Failed to list all chat sessions")
			return tmp.Error
		}

		// Create the wrapper objects
		for _, sessionEntry := range dbEntries {
			handleObject := c.defineSessionHandle(ctxt, sessionEntry)
			result = append(result, &handleObject)
		}

		return nil
	})
}

/*
GetSession fetch a session

	@param ctxt context.Context - query context
	@param sessionID string - session ID
	@return session entry
*/
func (c *sqlChatPersistance) GetSession(ctxt context.Context, sessionID string) (ChatSession, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	var result sqlChatSessionHandle
	return &result, c.db.Transaction(func(tx *gorm.DB) error {
		var entry sqlChatSessionEntry

		userID, err := c.user.GetID(ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to get associated user ID")
			return err
		}

		if tmp := tx.
			Where(&sqlChatSessionEntry{UserID: userID, ID: sessionID}).
			First(&entry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to query entry for session '%s'", sessionID)
			return tmp.Error
		}

		// Prepare wrapper object
		result = c.defineSessionHandle(ctxt, entry)
		return nil
	})
}

/*
CurrentActiveSession get the current active chat session for the associated user

	@param ctxt context.Context - query context
	@return session entry
*/
func (c *sqlChatPersistance) CurrentActiveSession(ctxt context.Context) (ChatSession, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	var result *sqlChatSessionHandle = nil
	return result, c.db.Transaction(func(tx *gorm.DB) error {
		var entry sqlChatSessionEntry

		activeSessionID, err := c.user.GetActiveSessionID(ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Failed to read user's active session ID")
		}

		if activeSessionID == nil {
			// The user has no active sessions
			log.WithFields(logtags).Debug("User has no active sessions")
			return nil
		}

		userID, err := c.user.GetID(ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to get associated user ID")
			return err
		}

		if tmp := tx.
			Where(&sqlChatSessionEntry{UserID: userID, ID: *activeSessionID}).
			First(&entry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to query entry for session '%s'", *activeSessionID)
			return tmp.Error
		}

		// Prepare wrapper object
		tmp := c.defineSessionHandle(ctxt, entry)
		result = &tmp
		return nil
	})
}

/*
SetActiveSession set the current active chat session for the associated user

	@param ctxt context.Context - query context
	@param session ChatSession - the chat session
*/
func (c *sqlChatPersistance) SetActiveSession(ctxt context.Context, session ChatSession) error {
	logtags := c.GetLogTagsForContext(ctxt)
	sessionID, err := session.SessionID(ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to read session ID")
		return err
	}
	return c.user.SetActiveSessionID(ctxt, sessionID)
}

/*
DeleteSession delete a session

	@param ctxt context.Context - query context
	@param sessionID string - session ID
*/
func (c *sqlChatPersistance) DeleteSession(ctxt context.Context, sessionID string) error {
	logtags := c.GetLogTagsForContext(ctxt)
	if err := c.db.Transaction(func(tx *gorm.DB) error {
		userID, err := c.user.GetID(ctxt)
		if err != nil {
			log.WithError(err).WithFields(logtags).Error("Unable to get associated user ID")
			return err
		}

		if tmp := tx.
			Where(&sqlChatSessionEntry{UserID: userID, ID: sessionID}).
			Delete(&sqlChatSessionEntry{}); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Unable to delete chat session '%s'", sessionID)
			return tmp.Error
		}
		return nil
	}); err != nil {
		return err
	}
	// In case the deleted session was the current active session for the user
	if err := c.user.Refresh(ctxt); err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to refresh user entry after session delete")
		return err
	}
	return nil
}

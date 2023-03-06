package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// sqlUserEntry SQL table representing a user
type sqlUserEntry struct {
	ID              string                `gorm:"primaryKey"`
	Name            string                `gorm:"not null;uniqueIndex:username_index"`
	APIToken        string                `gorm:"not null"`
	ActiveSessionID *string               `gorm:"default:null"`
	ActiveSession   *sqlChatSessionEntry  `gorm:"constraint:OnDelete:SET NULL;foreignKey:ActiveSessionID"`
	ChatSessions    []sqlChatSessionEntry `gorm:"foreignKey:UserID"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName hard code table name
func (sqlUserEntry) TableName() string {
	return "users"
}

// String override String
func (t sqlUserEntry) String() string {
	return fmt.Sprintf("(%s [%s])", t.Name, t.ID)
}

// sqlUserHandle wrapper object for working with the "users" table
type sqlUserHandle struct {
	goutils.Component
	driver *sqlUserPersistance
	sqlUserEntry
}

/*
GetID query for user GetID

	@param ctxt context.Context - query context
	@return the user ID
*/
func (h *sqlUserHandle) GetID(ctxt context.Context) (string, error) {
	return h.ID, nil
}

/*
GetName query for user name

	@param ctxt context.Context - query context
	@return the user name
*/
func (h *sqlUserHandle) GetName(ctxt context.Context) (string, error) {
	return h.Name, nil
}

/*
SetName set user name

	@param ctxt context.Context - query context
	@param newName string - new user name
*/
func (h *sqlUserHandle) SetName(ctxt context.Context, newName string) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.Model(&h.sqlUserEntry).Updates(&sqlUserEntry{Name: newName}).First(&h.sqlUserEntry)
		if tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to update user '%s' name to '%s'", h.ID, newName)
			return tmp.Error
		}
		return nil
	})
}

/*
GetActiveSessionID fetch user's active session ID

	@param ctxt context.Context - query context
	@return active session ID
*/
func (h *sqlUserHandle) GetActiveSessionID(ctxt context.Context) (*string, error) {
	return h.ActiveSessionID, nil
}

/*
SetActiveSessionID change user's active session ID

	@param ctxt context.Context - query context
	@param sessionID string - new session ID
*/
func (h *sqlUserHandle) SetActiveSessionID(ctxt context.Context, sessionID string) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.
			Model(&h.sqlUserEntry).
			Updates(&sqlUserEntry{ActiveSessionID: &sessionID}).
			First(&h.sqlUserEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to update user '%s' active session to '%s'", h.ID, sessionID)
			return tmp.Error
		}
		return nil
	})
}

/*
ClearActiveSessionID clear user's active session ID

	@param ctxt context.Context - query context
*/
func (h *sqlUserHandle) ClearActiveSessionID(ctxt context.Context) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.
			Model(&h.sqlUserEntry).
			Update("active_session_id", nil).
			First(&h.sqlUserEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to clear user '%s' active session", h.ID)
			return tmp.Error
		}
		return nil
	})
}

/*
GetAPIToken get user API token

	@param ctxt context.Context - query context
	@return the user API token
*/
func (h *sqlUserHandle) GetAPIToken(ctxt context.Context) (string, error) {
	return h.APIToken, nil
}

/*
Refresh helper function to sync the handler with what is stored in persistence

	@param ctxt context.Context - query context
*/
func (h *sqlUserHandle) Refresh(ctxt context.Context) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.
			Where(&sqlUserEntry{ID: h.ID}).
			First(&h.sqlUserEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to refresh user '%s' info", h.ID)
			return tmp.Error
		}
		return nil
	})
}

/*
SetAPIToken set user API token

	@param ctxt context.Context - query context
	@param newToken string - new API token
*/
func (h *sqlUserHandle) SetAPIToken(ctxt context.Context, newToken string) error {
	logtags := h.GetLogTagsForContext(ctxt)
	return h.driver.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.Model(&h.sqlUserEntry).Updates(&sqlUserEntry{APIToken: newToken}).First(&h.sqlUserEntry)
		if tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to update user '%s' API token", h.ID)
			return tmp.Error
		}
		return nil
	})
}

/*
ChatSessionManager fetch chat session manager for a user

	@param ctxt context.Context - query context
	@return associated chat session manager
*/
func (h *sqlUserHandle) ChatSessionManager(ctxt context.Context) (ChatSessionManager, error) {
	logtags := h.GetLogTagsForContext(ctxt)
	logtags["table"] = "chat_sessions"
	return &sqlChatPersistance{
		Component: goutils.Component{
			LogTags:         logtags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		},
		db:   h.driver.db,
		user: h,
	}, nil
}

// ============================================================================================
// SQL User Manager implementation

// defineUserHandle helper function for defining new user handle object
func (c *sqlUserPersistance) defineUserHandle(ctxt context.Context, userEntry sqlUserEntry) sqlUserHandle {
	logtags := c.GetLogTagsForContext(ctxt)
	logtags["table"] = "users"
	logtags["user"] = userEntry.String()
	return sqlUserHandle{
		Component: goutils.Component{
			LogTags:         logtags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		},
		driver:       c,
		sqlUserEntry: userEntry,
	}
}

/*
RecordNewUser record a new system user

	@param ctxt context.Context - query context
	@param userName string - user name
	@return	new user entry
*/
func (c *sqlUserPersistance) RecordNewUser(ctxt context.Context, userName string) (User, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	var result sqlUserHandle
	return &result, c.db.Transaction(func(tx *gorm.DB) error {
		log.WithFields(logtags).Debugf("Defining new user entry for '%s'", userName)

		// Define a new user entry
		newEntry := sqlUserEntry{ID: uuid.New().String(), Name: userName, APIToken: ""}
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Failed to define new entry for '%s'", userName)
			return tmp.Error
		}

		log.WithFields(logtags).Debugf("Defined new user entry for '%s'", userName)

		// Prepare wrapper object
		result = c.defineUserHandle(ctxt, newEntry)
		return nil
	})
}

/*
ListUsers list all known users

	@param ctxt context.Context - query context
	@return list of known users
*/
func (c *sqlUserPersistance) ListUsers(ctxt context.Context) ([]User, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	result := []User{}
	return result, c.db.Transaction(func(tx *gorm.DB) error {
		var dbEntries []sqlUserEntry

		if tmp := tx.Find(&dbEntries); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Error("Failed to list all users")
			return tmp.Error
		}

		// Create the wrapper objects
		for _, userEntry := range dbEntries {
			handleObject := c.defineUserHandle(ctxt, userEntry)
			result = append(result, &handleObject)
		}

		return nil
	})
}

/*
GetUser fetch a user

	@param ctxt context.Context - query context
	@param userID string - user ID
	@return user entry
*/
func (c *sqlUserPersistance) GetUser(ctxt context.Context, userID string) (User, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	var result sqlUserHandle
	return &result, c.db.Transaction(func(tx *gorm.DB) error {
		var dbEntry sqlUserEntry

		if tmp := tx.Where(&sqlUserEntry{ID: userID}).First(&dbEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Unable to locate user '%s'", userID)
			return tmp.Error
		}

		// Prepare wrapper object
		result = c.defineUserHandle(ctxt, dbEntry)
		return nil
	})
}

/*
GetUser fetch a user by name

	@param ctxt context.Context - query context
	@param userName string - user name
	@return user entry
*/
func (c *sqlUserPersistance) GetUserByName(ctxt context.Context, userName string) (User, error) {
	logtags := c.GetLogTagsForContext(ctxt)
	var result sqlUserHandle
	return &result, c.db.Transaction(func(tx *gorm.DB) error {
		var dbEntry sqlUserEntry

		if tmp := tx.Where(&sqlUserEntry{Name: userName}).First(&dbEntry); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Unable to locate user named '%s'", userName)
			return tmp.Error
		}

		// Prepare wrapper object
		result = c.defineUserHandle(ctxt, dbEntry)
		return nil
	})
}

/*
DeleteUser delete a user

	@param ctxt context.Context - query context
	@param userID string - user ID
*/
func (c *sqlUserPersistance) DeleteUser(ctxt context.Context, userID string) error {
	logtags := c.GetLogTagsForContext(ctxt)
	return c.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where(&sqlUserEntry{ID: userID}).Delete(&sqlUserEntry{}); tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logtags).
				Errorf("Unable to delete user '%s'", userID)
			return tmp.Error
		}

		return nil
	})
}

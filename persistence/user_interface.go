package persistence

import (
	"context"
)

/*
User holds information regarding one user of the system. This includes

  - User ID
  - User Name
  - User API token
*/
type User interface {
	/*
		GetID query for user GetID

			@param ctxt context.Context - query context
			@return the user ID
	*/
	GetID(ctxt context.Context) (string, error)

	/*
		GetName query for user name

			@param ctxt context.Context - query context
			@return the user name
	*/
	GetName(ctxt context.Context) (string, error)

	/*
		SetName set user name

			@param ctxt context.Context - query context
			@param newName string - new user name
	*/
	SetName(ctxt context.Context, newName string) error

	/*
		GetActiveSessionID fetch user's active session ID

			@param ctxt context.Context - query context
			@return active session ID
	*/
	GetActiveSessionID(ctxt context.Context) (*string, error)

	/*
	   SetActiveSessionID change user's active session ID

	   	@param ctxt context.Context - query context
	   	@param sessionID string - new session ID
	*/
	SetActiveSessionID(ctxt context.Context, sessionID string) error

	/*
	   ClearActiveSessionID clear user's active session ID

	   	@param ctxt context.Context - query context
	*/
	ClearActiveSessionID(ctxt context.Context) error

	/*
		GetAPIToken get user API token

			@param ctxt context.Context - query context
			@return the user API token
	*/
	GetAPIToken(ctxt context.Context) (string, error)

	/*
		SetAPIToken set user API token

			@param ctxt context.Context - query context
			@param newToken string - new API token
	*/
	SetAPIToken(ctxt context.Context, newToken string) error

	/*
		Refresh helper function to sync the handler with what is stored in persistence

			@param ctxt context.Context - query context
	*/
	Refresh(ctxt context.Context) error

	/*
		ChatSessionManager fetch chat session manager for a user

			@param ctxt context.Context - query context
			@return associated chat session manager
	*/
	ChatSessionManager(ctxt context.Context) (ChatSessionManager, error)
}

/*
UserManager user management client
*/
type UserManager interface {
	/*
		RecordNewUser record a new system user

			@param ctxt context.Context - query context
			@param userName string - user name
			@return	new user entry
	*/
	RecordNewUser(ctxt context.Context, userName string) (User, error)

	/*
		ListUsers list all known users

			@param ctxt context.Context - query context
			@return list of known users
	*/
	ListUsers(ctxt context.Context) ([]User, error)

	/*
		GetUser fetch a user

			@param ctxt context.Context - query context
			@param userID string - user ID
			@return user entry
	*/
	GetUser(ctxt context.Context, userID string) (User, error)

	/*
		GetUser fetch a user by name

			@param ctxt context.Context - query context
			@param userName string - user name
			@return user entry
	*/
	GetUserByName(ctxt context.Context, userName string) (User, error)

	/*
		DeleteUser delete a user

			@param ctxt context.Context - query context
			@param userID string - user ID
	*/
	DeleteUser(ctxt context.Context, userID string) error
}

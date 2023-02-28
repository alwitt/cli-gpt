package persistence

import (
	"context"
	"fmt"
	"testing"

	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestSQLChatManager(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := GetSQLUserManager(GetSqliteDialector(testDB))
	assert.Nil(err)

	utContext := context.Background()

	// Create test user
	user0, err := userManager.RecordNewUser(utContext, "unit-tester-0")
	assert.Nil(err)

	// Case 0: get chat manager
	chatMgmt0, err := user0.ChatSessionManager(utContext)
	assert.Nil(err)

	// Case 1: no sessions
	{
		sessions, err := chatMgmt0.ListSessions(utContext)
		assert.Nil(err)
		assert.Len(sessions, 0)
		_, err = chatMgmt0.GetSession(utContext, ulid.Make().String())
		assert.NotNil(err)
	}

	// Case 2: create new session
	model0 := uuid.NewString()
	session0, err := chatMgmt0.NewSession(utContext, model0)
	assert.Nil(err)
	sessionID0, err := session0.SessionID(utContext)
	assert.Nil(err)
	{
		aSession, err := chatMgmt0.GetSession(utContext, sessionID0)
		assert.Nil(err)
		model, err := aSession.CurrentModel(utContext)
		assert.Nil(err)
		assert.Equal(model0, model)
		sessions, err := chatMgmt0.ListSessions(utContext)
		assert.Nil(err)
		assert.Len(sessions, 1)
	}

	// Create another test user
	user1, err := userManager.RecordNewUser(utContext, "unit-tester-1")
	assert.Nil(err)
	chatMgmt1, err := user1.ChatSessionManager(utContext)
	assert.Nil(err)

	// Case 3: create new session
	model1 := uuid.NewString()
	session1, err := chatMgmt1.NewSession(utContext, model1)
	assert.Nil(err)
	sessionID1, err := session1.SessionID(utContext)
	assert.Nil(err)
	// Verify separation
	{
		aSession, err := chatMgmt1.GetSession(utContext, sessionID1)
		assert.Nil(err)
		model, err := aSession.CurrentModel(utContext)
		assert.Nil(err)
		assert.Equal(model1, model)
		sessions, err := chatMgmt1.ListSessions(utContext)
		assert.Nil(err)
		assert.Len(sessions, 1)
	}
	{
		_, err := chatMgmt0.GetSession(utContext, sessionID1)
		assert.NotNil(err)
		sessions, err := chatMgmt0.ListSessions(utContext)
		assert.Nil(err)
		assert.Len(sessions, 1)
	}

	// Case 4: delete session
	assert.Nil(chatMgmt1.DeleteSession(utContext, sessionID0))
	assert.Nil(chatMgmt0.DeleteSession(utContext, sessionID0))
	{
		_, err := chatMgmt0.GetSession(utContext, sessionID0)
		assert.NotNil(err)
		sessions, err := chatMgmt0.ListSessions(utContext)
		assert.Nil(err)
		assert.Len(sessions, 0)
	}
	{
		aSession, err := chatMgmt1.GetSession(utContext, sessionID1)
		assert.Nil(err)
		model, err := aSession.CurrentModel(utContext)
		assert.Nil(err)
		assert.Equal(model1, model)
		sessions, err := chatMgmt1.ListSessions(utContext)
		assert.Nil(err)
		assert.Len(sessions, 1)
	}
}

func TestSQLChatSession(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := GetSQLUserManager(GetSqliteDialector(testDB))
	assert.Nil(err)

	utContext := context.Background()

	// Create test user
	user0, err := userManager.RecordNewUser(utContext, "unit-tester-0")
	assert.Nil(err)

	// Create chat manager
	chatManager, err := user0.ChatSessionManager(utContext)
	assert.Nil(err)

	// Case 0: create session
	model0 := uuid.NewString()
	uut, err := chatManager.NewSession(utContext, model0)
	assert.Nil(err)
	{
		model, err := uut.CurrentModel(utContext)
		assert.Nil(err)
		assert.Equal(model0, model)
	}

	// Case 1: session state
	{
		theState, err := uut.SessionState(utContext)
		assert.Nil(err)
		assert.Equal(ChatSessionStateOpen, theState)
	}
	assert.Nil(uut.CloseSession(utContext))
	{
		theState, err := uut.SessionState(utContext)
		assert.Nil(err)
		assert.Equal(ChatSessionStateClose, theState)
	}

	// Case 2: user association
	{
		userEntry, err := uut.User(utContext)
		assert.Nil(err)
		readID, err := userEntry.GetID(utContext)
		assert.Nil(err)
		theID, err := user0.GetID(utContext)
		assert.Nil(err)
		assert.Equal(theID, readID)
	}

	// Case 2: session model
	model1 := uuid.NewString()
	assert.Nil(uut.ChangeModel(utContext, model1))
	{
		model, err := uut.CurrentModel(utContext)
		assert.Nil(err)
		assert.Equal(model1, model)
	}

	// Case 3: settings
	{
		settings, err := uut.Settings(utContext)
		assert.Nil(err)
		assert.Equal(DefaultChatMaxResponseTokens, settings.MaxTokens)
		assert.Nil(settings.Suffix)
		assert.Nil(settings.Temperature)
		assert.Nil(settings.TopP)
		assert.Nil(settings.Stop)
		assert.Nil(settings.PresencePenalty)
		assert.Nil(settings.FrequencyPenalty)
	}
	{
		newTemp := float32(0.398)
		newSetting := ChatSessionParameters{
			MaxTokens:   551,
			Temperature: &newTemp,
		}
		assert.Nil(uut.ChangeSettings(utContext, newSetting))
		settings, err := uut.Settings(utContext)
		assert.Nil(err)
		assert.Equal(551, settings.MaxTokens)
		assert.InDelta(newTemp, *settings.Temperature, 1e-6)
	}
	// Invalid setting
	{
		newFreqPen := float32(2.3)
		newSetting := ChatSessionParameters{
			MaxTokens:        1024,
			FrequencyPenalty: &newFreqPen,
		}
		assert.NotNil(uut.ChangeSettings(utContext, newSetting))
	}
}

package persistence

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/logger"
)

func TestSQLChatManager(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := GetSQLUserManager(GetSqliteDialector(testDB), logger.Info)
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

func TestSQLUserActiveSessionSet(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := GetSQLUserManager(GetSqliteDialector(testDB), logger.Info)
	assert.Nil(err)

	utContext := context.Background()

	// Create test user
	user0, err := userManager.RecordNewUser(utContext, "unit-tester-0")
	assert.Nil(err)

	// Case 0: no active session
	{
		activeSession, err := user0.GetActiveSessionID(utContext)
		assert.Nil(err)
		assert.Nil(activeSession)
	}

	// Case 1: unknown session
	assert.NotNil(user0.SetActiveSessionID(utContext, uuid.NewString()))

	// Create chat manager
	chatManager, err := user0.ChatSessionManager(utContext)
	assert.Nil(err)

	// Create session
	session0, err := chatManager.NewSession(utContext, uuid.NewString())
	assert.Nil(err)

	// Case 2: link session
	sessionID0, err := session0.SessionID(utContext)
	assert.Nil(err)
	assert.Nil(user0.SetActiveSessionID(utContext, sessionID0))
	{
		activeSession, err := user0.GetActiveSessionID(utContext)
		assert.Nil(err)
		assert.Equal(sessionID0, *activeSession)
	}

	// Case 3: check from the session manager side
	{
		activeSession, err := chatManager.CurrentActiveSession(utContext)
		assert.Nil(err)
		sessionID, err := activeSession.SessionID(utContext)
		assert.Nil(err)
		assert.Equal(sessionID0, sessionID)
	}

	// Create session
	session1, err := chatManager.NewSession(utContext, uuid.NewString())
	assert.Nil(err)

	// Case 4: link session through session manager
	assert.Nil(chatManager.SetActiveSession(utContext, session1))
	{
		sessionID1, err := session1.SessionID(utContext)
		assert.Nil(err)
		activeSession, err := user0.GetActiveSessionID(utContext)
		assert.Nil(err)
		assert.Equal(sessionID1, *activeSession)
	}
}

func TestSQLChatSession(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := GetSQLUserManager(GetSqliteDialector(testDB), logger.Info)
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

func TestSQLChatExchange(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := GetSQLUserManager(GetSqliteDialector(testDB), logger.Info)
	assert.Nil(err)

	utContext := context.Background()

	// Create test user
	user0, err := userManager.RecordNewUser(utContext, "unit-tester-0")
	assert.Nil(err)

	// Create chat manager
	chatManager, err := user0.ChatSessionManager(utContext)
	assert.Nil(err)

	// Create session
	model0 := uuid.NewString()
	uut, err := chatManager.NewSession(utContext, model0)
	assert.Nil(err)
	{
		model, err := uut.CurrentModel(utContext)
		assert.Nil(err)
		assert.Equal(model0, model)
	}

	currentTime := time.Now()
	timeDelta := time.Second * 5

	// Case 0: record exchange
	exchange0 := ChatExchange{
		RequestTimestamp:  currentTime,
		Request:           fmt.Sprintf("req-0-%s", uuid.NewString()),
		ResponseTimestamp: currentTime.Add(timeDelta),
		Response:          fmt.Sprintf("resp-0-%s", uuid.NewString()),
	}
	assert.Nil(uut.RecordOneExchange(utContext, exchange0))
	{
		exchanges, err := uut.Exchanges(utContext)
		assert.Nil(err)
		assert.Len(exchanges, 1)
		assert.Equal(exchange0.Request, exchanges[0].Request)
		assert.Equal(exchange0.Response, exchanges[0].Response)
	}

	// Case 1: record exchange
	currentTime = currentTime.Add(timeDelta)
	exchange1 := ChatExchange{
		RequestTimestamp:  currentTime,
		Request:           fmt.Sprintf("req-1-%s", uuid.NewString()),
		ResponseTimestamp: currentTime.Add(timeDelta),
		Response:          fmt.Sprintf("resp-1-%s", uuid.NewString()),
	}
	assert.Nil(uut.RecordOneExchange(utContext, exchange1))
	{
		exchanges, err := uut.Exchanges(utContext)
		assert.Nil(err)
		assert.Len(exchanges, 2)
		assert.Equal(exchange1.Request, exchanges[1].Request)
		assert.Equal(exchange1.Response, exchanges[1].Response)
	}
	{
		firstExchange, err := uut.FirstExchange(utContext)
		assert.Nil(err)
		assert.Equal(exchange0.Request, firstExchange.Request)
		assert.Equal(exchange0.Response, firstExchange.Response)
	}

	// Case 2: record earlier exchange
	currentTime = currentTime.Add(timeDelta * -4)
	exchange2 := ChatExchange{
		RequestTimestamp:  currentTime,
		Request:           fmt.Sprintf("req-2-%s", uuid.NewString()),
		ResponseTimestamp: currentTime.Add(timeDelta),
		Response:          fmt.Sprintf("resp-2-%s", uuid.NewString()),
	}
	assert.Nil(uut.RecordOneExchange(utContext, exchange2))
	{
		exchanges, err := uut.Exchanges(utContext)
		assert.Nil(err)
		assert.Len(exchanges, 3)
		assert.Equal(exchange1.Request, exchanges[2].Request)
		assert.Equal(exchange1.Response, exchanges[2].Response)
	}
	{
		firstExchange, err := uut.FirstExchange(utContext)
		assert.Nil(err)
		assert.Equal(exchange2.Request, firstExchange.Request)
		assert.Equal(exchange2.Response, firstExchange.Response)
	}
}

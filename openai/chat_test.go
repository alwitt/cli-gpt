package openai

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alwitt/cli-gpt/mocks"
	"github.com/alwitt/cli-gpt/persistence"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChatSessionHandler(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	// Define mock objects
	mockUser := new(mocks.User)
	mockClient := new(mocks.Client)
	mockChatSession := new(mocks.ChatSession)
	mockPromptBuilder := new(mocks.ChatPromptBuilder)

	utContext := context.Background()

	// Setup default responses
	mockChatSession.On("User", utContext).Return(mockUser, nil)
	mockUser.On("GetName", utContext).Return("unit-tester", nil)
	mockChatSession.On("SessionID", utContext).Return(uuid.NewString(), nil)

	// Create new chat session handler
	uut, err := DefineChatSessionHandler(utContext, mockChatSession, mockClient, mockPromptBuilder)
	assert.Nil(err)

	// Case 0: normal flow
	{
		testPrompt := uuid.NewString()
		testFullPrompt := uuid.NewString()
		testResponse := uuid.NewString()
		testRespChan := make(chan string)

		// Setup mocks
		mockChatSession.
			On("SessionState", utContext).
			Return(persistence.ChatSessionStateOpen, nil).
			Once()
		mockPromptBuilder.
			On("CreatePrompt", utContext, mockChatSession, testPrompt).
			Return(testFullPrompt, nil).
			Once()
		mockClient.On(
			"MakeCompletionRequest",
			mock.AnythingOfType("*context.cancelCtx"),
			mockChatSession,
			testFullPrompt,
			mock.AnythingOfType("chan string"),
		).Run(func(args mock.Arguments) {
			respChan := args.Get(3).(chan string)
			defer close(respChan)
			respChan <- testResponse
		}).Return(nil).Once()
		mockChatSession.On(
			"RecordOneExchange",
			utContext,
			mock.AnythingOfType("persistence.ChatExchange"),
		).Run(func(args mock.Arguments) {
			newExchange := args.Get(1).(persistence.ChatExchange)
			assert.Equal(testPrompt, newExchange.Request)
			assert.Equal(testResponse, newExchange.Response)
		}).Return(nil).Once()

		// Make request
		wg := sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.Nil(uut.SendRequest(utContext, testPrompt, testRespChan))
		}()

		// Read expected response
		select {
		case <-time.After(time.Millisecond * 10):
			assert.NotNilf(nil, "timeout reading for response")
		case rxMsg, ok := <-testRespChan:
			assert.True(ok)
			assert.Equal(testResponse, rxMsg)
		}
	}

	// Case 1: wrong session state
	{
		testPrompt := uuid.NewString()
		testRespChan := make(chan string)

		// Setup mocks
		mockChatSession.
			On("SessionState", utContext).
			Return(persistence.ChatSessionStateClose, nil).
			Once()

		assert.NotNil(uut.SendRequest(utContext, testPrompt, testRespChan))
	}

	// Case 2: prompt build failure
	{
		testPrompt := uuid.NewString()
		testRespChan := make(chan string)

		// Setup mocks
		mockChatSession.
			On("SessionState", utContext).
			Return(persistence.ChatSessionStateOpen, nil).
			Once()
		mockPromptBuilder.
			On("CreatePrompt", utContext, mockChatSession, testPrompt).
			Return("", fmt.Errorf("dummy error")).
			Once()

		assert.NotNil(uut.SendRequest(utContext, testPrompt, testRespChan))
	}

	// Case 3: client ended request with error
	{
		testPrompt := uuid.NewString()
		testFullPrompt := uuid.NewString()
		testRespChan := make(chan string)

		// Setup mocks
		mockChatSession.
			On("SessionState", utContext).
			Return(persistence.ChatSessionStateOpen, nil).
			Once()
		mockPromptBuilder.
			On("CreatePrompt", utContext, mockChatSession, testPrompt).
			Return(testFullPrompt, nil).
			Once()
		mockClient.On(
			"MakeCompletionRequest",
			mock.AnythingOfType("*context.cancelCtx"),
			mockChatSession,
			testFullPrompt,
			mock.AnythingOfType("chan string"),
		).Return(fmt.Errorf("dummy error")).Once()

		// Make request
		wg := sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NotNil(uut.SendRequest(utContext, testPrompt, testRespChan))
		}()

		// Read expected response
		select {
		case <-time.After(time.Millisecond * 10):
			assert.NotNilf(nil, "timeout reading for response")
		case _, ok := <-testRespChan:
			assert.False(ok, "unexpected response")
		}
	}

	// Case 4: normal flow, but client returned nothing
	{
		testPrompt := uuid.NewString()
		testFullPrompt := uuid.NewString()
		testRespChan := make(chan string)

		// Setup mocks
		mockChatSession.
			On("SessionState", utContext).
			Return(persistence.ChatSessionStateOpen, nil).
			Once()
		mockPromptBuilder.
			On("CreatePrompt", utContext, mockChatSession, testPrompt).
			Return(testFullPrompt, nil).
			Once()
		mockClient.On(
			"MakeCompletionRequest",
			mock.AnythingOfType("*context.cancelCtx"),
			mockChatSession,
			testFullPrompt,
			mock.AnythingOfType("chan string"),
		).Run(func(args mock.Arguments) {
			respChan := args.Get(3).(chan string)
			defer close(respChan)
		}).Return(nil).Once()
		mockChatSession.On(
			"RecordOneExchange",
			utContext,
			mock.AnythingOfType("persistence.ChatExchange"),
		).Run(func(args mock.Arguments) {
			newExchange := args.Get(1).(persistence.ChatExchange)
			assert.Equal(testPrompt, newExchange.Request)
			assert.Empty(newExchange.Response)
		}).Return(nil).Once()

		// Make request
		wg := sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.Nil(uut.SendRequest(utContext, testPrompt, testRespChan))
		}()

		// Read expected response
		select {
		case <-time.After(time.Millisecond * 10):
			assert.NotNilf(nil, "timeout reading for response")
		case _, ok := <-testRespChan:
			assert.False(ok, "unexpected response")
		}
	}
}

package api

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/alwitt/goutils"
	"github.com/apex/log"
)

/*
ChatSessionHandler represents a chat session
*/
type ChatSessionHandler interface {
	/*
		SendRequest send a new request within the session

			@param ctxt context.Context - query context
			@param prompt string - the prompt to send
			@param resp chan string - channel for sending out the responses from the model
	*/
	SendRequest(ctxt context.Context, prompt string, resp chan string) error

	/*
		Close close this session

			@param ctxt context.Context - query context
	*/
	Close(ctxt context.Context) error
}

// chatSessionHandlerImpl implements ChatSessionHandler
type chatSessionHandlerImpl struct {
	goutils.Component
	session       persistence.ChatSession
	client        Client
	promptBuilder ChatPromptBuilder
}

/*
DefineChatSessionHandler create chat session tracker

	@param ctxt context.Context - query context
	@param session persistence.ChatSession - chat session parameters
	@param client GPTClient - OpenAI GPT model API client
	@param promptBuilder PromptBuilder - prompt builder to use
	@return new chat session tracker
*/
func DefineChatSessionHandler(
	ctxt context.Context,
	session persistence.ChatSession,
	client Client,
	promptBuilder ChatPromptBuilder,
) (ChatSessionHandler, error) {
	user, err := session.User(ctxt)
	if err != nil {
		log.WithError(err).Error("Failed to read user object")
		return nil, err
	}
	userName, err := user.GetName(ctxt)
	if err != nil {
		log.WithError(err).Error("Failed to read user name")
		return nil, err
	}
	sessionID, err := session.SessionID(ctxt)
	if err != nil {
		log.WithError(err).Error("Failed to read session ID")
		return nil, err
	}

	logTags := log.Fields{
		"module": "openai", "component": "chat-session-handler", "user": userName, "session": sessionID,
	}
	return &chatSessionHandlerImpl{
		Component: goutils.Component{
			LogTags:         logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		}, session: session, client: client, promptBuilder: promptBuilder,
	}, nil
}

/*
SendRequest send a new request within the session

	@param ctxt context.Context - query context
	@param prompt string - the prompt to send
	@param resp chan string - channel for sending out the responses from the model
*/
func (s *chatSessionHandlerImpl) SendRequest(ctxt context.Context, prompt string, resp chan string) error {
	logtags := s.GetLogTagsForContext(ctxt)
	defer close(resp)

	// Verify state is correct
	currentState, err := s.session.SessionState(ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Unable to read session state")
		return err
	}
	if currentState != persistence.ChatSessionStateOpen {
		err := fmt.Errorf("chat session is closed")
		log.
			WithError(err).
			WithFields(logtags).
			Error("Session state does not allow new requests")
		return err
	}

	// Prepare a separate channel for receiving responses from the client
	clientResp := make(chan string)

	// Build the request prompt
	actualPrompt, err := s.promptBuilder.CreatePrompt(ctxt, s.session, prompt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to build new complete prompt")
		return err
	}

	wg := sync.WaitGroup{}
	defer wg.Wait()

	requestCtxt, ctxtCancel := context.WithCancel(ctxt)
	defer ctxtCancel()

	log.WithFields(logtags).Debug("Starting new request")

	// Make the request
	var requestErr error
	requestErr = nil
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.client.MakeCompletionRequest(requestCtxt, s.session, actualPrompt, clientResp)
		if err != nil {
			requestErr = err
			ctxtCancel()
		}
	}()
	requestTimestamp := time.Now()

	rg := regexp.MustCompile(`(\r\n?|\n){2,}`)

	// Process the received response segments
	respBuilder := strings.Builder{}
	complete := false
	for !complete {
		select {
		case msg, ok := <-clientResp:
			if ok {
				// Clean newline
				cleaned := rg.ReplaceAllString(msg, "$1")
				// New response segment
				respBuilder.WriteString(cleaned)
				// Pass up to caller
				resp <- msg
			} else {
				// channel is close
				complete = true
			}
		case <-requestCtxt.Done():
			complete = true
		}
	}

	if requestErr != nil {
		log.WithError(requestErr).WithFields(logtags).Error("Request call failed with error")
		return requestErr
	}

	responseTimestamp := time.Now()
	response := respBuilder.String()

	log.WithFields(logtags).Debug("Received full response")

	// Record this exchange
	exchange := persistence.ChatExchange{
		RequestTimestamp:  requestTimestamp,
		Request:           prompt,
		ResponseTimestamp: responseTimestamp,
		Response:          response,
	}

	if err := s.session.RecordOneExchange(ctxt, exchange); err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to record new exchange")
		return err
	}

	return nil
}

/*
Close close this session

	@param ctxt context.Context - query context
*/
func (s *chatSessionHandlerImpl) Close(ctxt context.Context) error {
	return s.session.CloseSession(ctxt)
}

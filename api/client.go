package api

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/alwitt/goutils"
	"github.com/apex/log"
	openai "github.com/sashabaranov/go-openai"
)

/*
Client OpenAI model API client
*/
type Client interface {
	/*
		MakeCompletionRequest make a completion request to the model

			@param ctxt context.Context - query context
			@param session persistence.ChatSession - chat session parameters
			@param prompt string - the prompt to send
			@param resp chan string - channel for sending out the responses from the model
	*/
	MakeCompletionRequest(
		ctxt context.Context, session persistence.ChatSession, prompt string, resp chan string,
	) error
}

// clientImpl implements Client
type clientImpl struct {
	goutils.Component
	client  *openai.Client
	builder ChatPromptBuilder
}

/*
GetClient define new OpenA model API client

	@param ctxt context.Context - query context
	@param user persistence.User - the user parameter
	@param promptBuilder ChatPromptBuilder - tool to construct a complete prompt for models
	    whose input do not have a way to define user request and system response
	@return client
*/
func GetClient(
	ctxt context.Context, user persistence.User, promptBuilder ChatPromptBuilder,
) (Client, error) {
	userName, err := user.GetName(ctxt)
	if err != nil {
		log.WithError(err).Error("Failed to read user name")
		return nil, err
	}
	userAPI, err := user.GetAPIToken(ctxt)
	if err != nil {
		log.WithError(err).Error("Failed to read user API token")
		return nil, err
	}

	logTags := log.Fields{"module": "openai", "component": "client", "user": userName}
	return &clientImpl{
		Component: goutils.Component{
			LogTags:         logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		},
		client:  openai.NewClient(userAPI),
		builder: promptBuilder,
	}, nil
}

/*
MakeCompletionRequest make a completion request to the model

	@param ctxt context.Context - query context
	@param session persistence.ChatSession - chat session parameters
	@param prompt string - the prompt to send
	@param resp chan string - channel for sending out the responses from the model
*/
func (c *clientImpl) MakeCompletionRequest(
	ctxt context.Context, session persistence.ChatSession, prompt string, resp chan string,
) error {
	logtags := c.GetLogTagsForContext(ctxt)

	sessionID, err := session.SessionID(ctxt)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			WithField("request_type", "completion").
			Error("Unable to read session ID to start request")
		return err
	}
	logtags["session"] = sessionID

	sessionState, err := session.SessionState(ctxt)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			WithField("request_type", "completion").
			Error("Unable to read session state to start request")
		return err
	}
	if sessionState != persistence.ChatSessionStateOpen {
		err := fmt.Errorf("chat session is closed")
		log.
			WithError(err).
			WithFields(logtags).
			WithField("request_type", "completion").
			Error("Session state does not allow new requests")
		return err
	}

	settings, err := session.Settings(ctxt)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			WithField("request_type", "completion").
			Error("Unable to read session settings to start request")
		return err
	}

	if settings.Model == "turbo" {
		return c.makeChatCompletionRequest(ctxt, session, settings, prompt, resp)
	}
	return c.makeTextCompletionRequest(ctxt, session, settings, prompt, resp)
}

/*
makeTextCompletionRequest make a text completion request to the model

	@param ctxt context.Context - query context
	@param session persistence.ChatSession - chat session parameters
	@param settings persistence.ChatSessionParameters - session settings
	@param prompt string - the prompt to send
	@param resp chan string - channel for sending out the responses from the model
*/
func (c *clientImpl) makeTextCompletionRequest(
	ctxt context.Context,
	session persistence.ChatSession,
	settings persistence.ChatSessionParameters,
	prompt string,
	resp chan string,
) error {
	logtags := c.GetLogTagsForContext(ctxt)

	// Determine the model to use
	var requestedModel string
	switch settings.Model {
	case "davinci":
		requestedModel = openai.GPT3TextDavinci003
	case "curie":
		requestedModel = openai.GPT3TextCurie001
	case "babbage":
		requestedModel = openai.GPT3TextBabbage001
	case "ada":
		requestedModel = openai.GPT3TextAda001
	default:
		requestedModel = openai.GPT3TextDavinci003
	}

	actualPrompt, err := c.builder.CreatePrompt(ctxt, session, prompt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Failed to build new complete prompt")
		return err
	}

	// Build the request
	request := openai.CompletionRequest{
		Model:     requestedModel,
		MaxTokens: settings.MaxTokens,
		Prompt:    actualPrompt,
		Stream:    true,
	}
	// Apply optional settings
	if settings.Suffix != nil {
		request.Suffix = *settings.Suffix
	}
	if settings.Temperature != nil {
		request.Temperature = *settings.Temperature
	}
	if settings.TopP != nil {
		request.TopP = *settings.TopP
	}
	if len(settings.Stop) > 0 {
		request.Stop = settings.Stop
	}
	if settings.PresencePenalty != nil {
		request.PresencePenalty = *settings.PresencePenalty
	}
	if settings.FrequencyPenalty != nil {
		request.FrequencyPenalty = *settings.FrequencyPenalty
	}

	stream, err := c.client.CreateCompletionStream(ctxt, request)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			WithField("request_type", "completion").
			Error("Failed to start request")
		return err
	}
	defer stream.Close()

	log.
		WithFields(logtags).
		WithField("request_type", "completion").
		Debugf("Starting new request to model '%s'", requestedModel)

	defer close(resp)
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				WithField("request_type", "completion").
				Error("Response stream read failed")
			return err
		}

		if len(response.Choices) > 0 {
			// Return the response to the caller
			resp <- response.Choices[0].Text
		}
	}

	log.
		WithFields(logtags).
		WithField("request_type", "completion").
		Debugf("Request complete")
	return nil
}

/*
makeChatCompletionRequest make a chat completion request to the model

This is only meant to be used with the Turbo model "gpt-3.5-turbo" (i.e. ChatGPT)

	@param ctxt context.Context - query context
	@param session persistence.ChatSession - chat session parameters
	@param settings persistence.ChatSessionParameters - session settings
	@param prompt string - the prompt to send
	@param resp chan string - channel for sending out the responses from the model
*/
func (c *clientImpl) makeChatCompletionRequest(
	ctxt context.Context,
	session persistence.ChatSession,
	settings persistence.ChatSessionParameters,
	prompt string,
	resp chan string,
) error {
	logtags := c.GetLogTagsForContext(ctxt)

	requestedModel := openai.GPT3Dot5Turbo

	// Build the request
	request := openai.ChatCompletionRequest{
		Model:     requestedModel,
		MaxTokens: settings.MaxTokens,
		Stream:    true,
	}
	// Apply optional settings
	if settings.Temperature != nil {
		request.Temperature = *settings.Temperature
	}
	if settings.TopP != nil {
		request.TopP = *settings.TopP
	}
	if len(settings.Stop) > 0 {
		request.Stop = settings.Stop
	}
	if settings.PresencePenalty != nil {
		request.PresencePenalty = *settings.PresencePenalty
	}
	if settings.FrequencyPenalty != nil {
		request.FrequencyPenalty = *settings.FrequencyPenalty
	}

	// Define request messages
	requestMsgs := []openai.ChatCompletionMessage{
		{Role: "system", Content: "You are a helpful assistant."},
	}
	exchanges, err := session.Exchanges(ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Unable to fetch session exchanges")
		return err
	}
	for _, oneExchange := range exchanges {
		requestMsgs = append(requestMsgs, []openai.ChatCompletionMessage{
			{Role: "user", Content: oneExchange.Request},
			{Role: "assistant", Content: oneExchange.Response},
		}...)
	}
	request.Messages = requestMsgs
	request.Messages = append(
		request.Messages, openai.ChatCompletionMessage{Role: "user", Content: prompt},
	)

	stream, err := c.client.CreateChatCompletionStream(ctxt, request)
	if err != nil {
		log.
			WithError(err).
			WithFields(logtags).
			WithField("request_type", "completion").
			Error("Failed to start request")
		return err
	}
	defer stream.Close()

	log.
		WithFields(logtags).
		WithField("request_type", "completion").
		Debugf("Starting new request to model '%s'", requestedModel)

	defer close(resp)
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			log.
				WithError(err).
				WithFields(logtags).
				WithField("request_type", "completion").
				Error("Response stream read failed")
			return err
		}

		if len(response.Choices) > 0 {
			// Return the response to the caller
			if response.Choices[0].FinishReason == "content_filter" {
				err := fmt.Errorf("request blocked by content filter")
				return err
			} else if response.Choices[0].FinishReason == "length" {
				err := fmt.Errorf("max request length exceeded")
				return err
			}
			resp <- response.Choices[0].Delta.Content
		}
	}

	log.
		WithFields(logtags).
		WithField("request_type", "completion").
		Debugf("Request complete")
	return nil
}

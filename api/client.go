package api

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/alwitt/goutils"
	"github.com/apex/log"
	gogpt "github.com/sashabaranov/go-gpt3"
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
	client *gogpt.Client
}

/*
GetClient define new OpenA model API client

	@param ctxt context.Context - query context
	@param user persistence.User - the user parameter
	@return client
*/
func GetClient(ctxt context.Context, user persistence.User) (Client, error) {
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
		client: gogpt.NewClient(userAPI),
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

	// Determine the model to use
	var requestedModel string
	switch settings.Model {
	case "davinci":
		requestedModel = gogpt.GPT3TextDavinci003
	case "curie":
		requestedModel = gogpt.GPT3TextCurie001
	case "babbage":
		requestedModel = gogpt.GPT3TextBabbage001
	case "ada":
		requestedModel = gogpt.GPT3TextAda001
	default:
		requestedModel = gogpt.GPT3TextDavinci003
	}

	// Build the request
	request := gogpt.CompletionRequest{
		Model:     requestedModel,
		MaxTokens: settings.MaxTokens,
		Prompt:    prompt,
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

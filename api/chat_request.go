package api

import (
	"context"
	"strings"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/alwitt/goutils"
	"github.com/apex/log"
)

/*
ChatPromptBuilder construct a text completion prompt to send
*/
type ChatPromptBuilder interface {
	/*
		CreatePrompt build a complete prompt using the existing session exchanges, and the new
		request from the user.

			@param ctxt context.Context - query context
			@param session persistence.ChatSession - current chat session
			@param newRequest string - new user request
			@return complete prompt for the text completion model
	*/
	CreatePrompt(
		ctxt context.Context, session persistence.ChatSession, newRequest string,
	) (string, error)
}

// concatenateChatPromptBuilder build a prompt by concatenating the request and responses together
type concatenateChatPromptBuilder struct {
	goutils.Component
}

/*
GetSimpleChatPromptBuilder define a simple chat prompt builder
*/
func GetSimpleChatPromptBuilder() (ChatPromptBuilder, error) {
	logTags := log.Fields{
		"module": "openai", "component": "prompt-builder", "instance": "plain-concatenate",
	}
	return &concatenateChatPromptBuilder{
		Component: goutils.Component{
			LogTags:         logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		},
	}, nil
}

/*
CreatePrompt build a complete prompt using the existing session exchanges, and the new
request from the user.

	@param ctxt context.Context - query context
	@param session persistence.ChatSession - current chat session
	@param newRequest string - new user request
	@return complete prompt for the text completion model
*/
func (b *concatenateChatPromptBuilder) CreatePrompt(
	ctxt context.Context, session persistence.ChatSession, newRequest string,
) (string, error) {
	logtags := b.GetLogTagsForContext(ctxt)

	allExchanges, err := session.Exchanges(ctxt)
	if err != nil {
		log.WithError(err).WithFields(logtags).Error("Unable to query for all session exchanges")
		return "", err
	}

	fullPromptBuilder := strings.Builder{}

	// Pull together the existing exchanges
	for _, oneExchange := range allExchanges {
		for _, entry := range []string{oneExchange.Request, "\n\n", oneExchange.Response, "\n\n"} {
			if _, err := fullPromptBuilder.WriteString(entry); err != nil {
				log.WithError(err).WithFields(logtags).Error("Request concatenation failed")
				return "", err
			}
		}
	}

	// Add the new request
	if _, err := fullPromptBuilder.WriteString(newRequest); err != nil {
		log.WithError(err).WithFields(logtags).Error("Request concatenation failed")
		return "", err
	}

	return fullPromptBuilder.String(), nil
}

package openai

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alwitt/cli-gpt/persistence"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestConcatenateChatPromptBuilder(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	userManager, err := persistence.GetSQLUserManager(persistence.GetSqliteDialector(testDB))
	assert.Nil(err)

	utContext := context.Background()

	// Create test user
	user0, err := userManager.RecordNewUser(utContext, "unit-tester-0")
	assert.Nil(err)

	// Create chat manager
	chatManager, err := user0.ChatSessionManager(utContext)
	assert.Nil(err)

	// Define exchanges
	chatSession, err := chatManager.NewSession(utContext, uuid.NewString())
	assert.Nil(err)
	exchanges := []persistence.ChatExchange{}
	{
		currentTime := time.Now()
		timeDelta := time.Second * 5
		for itr := 0; itr < 3; itr++ {
			exchanges = append(exchanges, persistence.ChatExchange{
				RequestTimestamp:  currentTime,
				Request:           fmt.Sprintf("req-%d-%s", itr, uuid.NewString()),
				ResponseTimestamp: currentTime.Add(timeDelta),
				Response:          fmt.Sprintf("resp-%d-%s", itr, uuid.NewString()),
			})
			currentTime = currentTime.Add(timeDelta)
		}
	}
	for _, oneExchange := range exchanges {
		assert.Nil(chatSession.RecordOneExchange(utContext, oneExchange))
	}

	// Build a prompt
	uut, err := GetSimpleChatPromptBuilder()
	assert.Nil(err)

	fullPrompt, err := uut.CreatePrompt(utContext, chatSession, "Hello World")
	assert.Nil(err)
	log.Debugf("Complete prompt:\n%s", fullPrompt)

	{
		builder := strings.Builder{}
		for _, exchange := range exchanges {
			for _, entry := range []string{exchange.Request, "\n\n", exchange.Response, "\n\n"} {
				builder.WriteString(entry)
			}
		}
		builder.WriteString("Hello World")
		expectedPrompt := builder.String()
		assert.Equal(expectedPrompt, fullPrompt)
	}
}

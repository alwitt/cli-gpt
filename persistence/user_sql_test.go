package persistence

import (
	"context"
	"fmt"
	"testing"

	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSQLUserManager(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	uut, err := GetSQLUserManager(GetSqliteDialector(testDB))
	assert.Nil(err)

	utContext := context.Background()

	// Case 0: no users
	{
		allUsers, err := uut.ListUsers(utContext)
		assert.Nil(err)
		assert.Len(allUsers, 0)
	}
	{
		_, err := uut.GetUser(utContext, uuid.NewString())
		assert.NotNil(err)
		_, err = uut.GetUserByName(utContext, uuid.NewString())
		assert.NotNil(err)
	}

	// Case 1: create new user
	userID0 := ""
	userName0 := uuid.NewString()
	{
		entry, err := uut.RecordNewUser(utContext, userName0)
		assert.Nil(err)
		readName, err := entry.GetName(utContext)
		assert.Nil(err)
		assert.Equal(userName0, readName)
		userID0, err = entry.GetID(utContext)
		assert.Nil(err)
	}
	{
		entry, err := uut.GetUserByName(utContext, userName0)
		assert.Nil(err)
		readID, err := entry.GetID(utContext)
		assert.Nil(err)
		assert.Equal(userID0, readID)
	}

	// Case 2: create user with same name again
	{
		_, err := uut.RecordNewUser(utContext, userName0)
		assert.NotNil(err)
	}
	{
		allUsers, err := uut.ListUsers(utContext)
		assert.Nil(err)
		assert.Len(allUsers, 1)
		id, err := allUsers[0].GetID(utContext)
		assert.Nil(err)
		name, err := allUsers[0].GetName(utContext)
		assert.Nil(err)
		assert.Equal(userID0, id)
		assert.Equal(userName0, name)
	}

	// Case 3: delete user
	assert.Nil(uut.DeleteUser(utContext, userID0))
	{
		_, err := uut.GetUser(utContext, userID0)
		assert.NotNil(err)
	}
}

func TestSQLUserEntryCRUD(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)

	uut, err := GetSQLUserManager(GetSqliteDialector(testDB))
	assert.Nil(err)

	utContext := context.Background()

	// Case 0: create new user
	userName0 := uuid.NewString()
	userEntry, err := uut.RecordNewUser(utContext, userName0)
	assert.Nil(err)
	{
		name, err := userEntry.GetName(utContext)
		assert.Nil(err)
		assert.Equal(userName0, name)
		api, err := userEntry.GetAPIToken(utContext)
		assert.Nil(err)
		assert.Empty(api)
	}

	// Case 1: change username
	userName1 := uuid.NewString()
	assert.Nil(userEntry.SetName(utContext, userName1))
	{
		name, err := userEntry.GetName(utContext)
		assert.Nil(err)
		assert.Equal(userName1, name)
	}

	// Case 2: change API token
	newAPI := uuid.NewString()
	assert.Nil(userEntry.SetAPIToken(utContext, newAPI))
	{
		api, err := userEntry.GetAPIToken(utContext)
		assert.Nil(err)
		assert.Equal(newAPI, api)
	}
}

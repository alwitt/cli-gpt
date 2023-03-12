package persistence

import (
	"fmt"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQL persistence layer driver
type sqlUserPersistence struct {
	goutils.Component
	db *gorm.DB
}

// SQL persistence layer driver specific to chat session management
type sqlChatPersistence struct {
	goutils.Component
	db   *gorm.DB
	user User
}

/*
GetSqliteDialector define Sqlite GORM dialector

	@param dbFile string - Sqlite DB file
	@return GORM sqlite dialector
*/
func GetSqliteDialector(dbFile string) gorm.Dialector {
	return sqlite.Open(fmt.Sprintf("%s?_foreign_keys=on", dbFile))
}

/*
GetSQLUserManager define a new SQL based user manager

	@param dbDialector gorm.Dialector - GORM SQL dialector
	@param logLevel logger.LogLevel - SQL log level
*/
func GetSQLUserManager(dbDialector gorm.Dialector, logLevel logger.LogLevel) (UserManager, error) {
	db, err := gorm.Open(dbDialector, &gorm.Config{
		Logger:                 logger.Default.LogMode(logLevel),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// Prepare the databases
	if err := db.AutoMigrate(&sqlUserEntry{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&sqlChatSessionEntry{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&sqlChatExchangeEntry{}); err != nil {
		return nil, err
	}

	logTags := log.Fields{"module": "persistence", "component": "user-manager", "instance": "sql"}
	return &sqlUserPersistence{
		Component: goutils.Component{
			LogTags:         logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		}, db: db,
	}, nil
}

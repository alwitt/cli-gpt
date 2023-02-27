package persistence

import (
	"github.com/alwitt/goutils"
	"github.com/apex/log"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQL persistance layer driver
type sqlPersistance struct {
	goutils.Component
	db *gorm.DB
}

/*
GetSqliteDialector define Sqlite GORM dialector

	@param dbFile string - Sqlite DB file
	@return GORM sqlite dialector
*/
func GetSqliteDialector(dbFile string) gorm.Dialector {
	return sqlite.Open(dbFile)
}

/*
GetSQLUserManager define a new SQL based user manager
*/
func GetSQLUserManager(dbDialector gorm.Dialector) (UserManager, error) {
	db, err := gorm.Open(dbDialector, &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Info),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// Prepare the databases
	if err := db.AutoMigrate(&sqlUserEntry{}); err != nil {
		return nil, err
	}

	logTags := log.Fields{"module": "persistence", "component": "user-manager", "instance": "sql"}
	return &sqlPersistance{
		Component: goutils.Component{
			LogTags:         logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{},
		}, db: db,
	}, nil
}

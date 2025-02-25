package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"time"
)

var DB *gorm.DB

func InitDB(dsn string) error {
	var err error
        gorm.Open(postgres.Open(dsn), &gorm.Config{})
	// Initialize GORM with PostgreSQL driver
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return err
	}

	// Get generic database object and configure pool
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	// Connection pool settings
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Auto migrate models with UUID extension
	err = DB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error
	if err != nil {
		return err
	}

	// Migrate all models
	err = DB.AutoMigrate(
		&UserModel{},
		&Prompt{},
		&AudioPromptModel{},
		&OTPModel{}, 
	)

	return err
}

func CloseDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() {
	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Error connecting to database: %v\n", err)
	}

	// Auto-migrate the schema
	err = DB.AutoMigrate(&User{}, &Device{}, &Signal{}, &SignalValue{})
	if err != nil {
		log.Fatalf("Error migrating database: %v\n", err)
	}

	log.Println("Database connection established and migrations completed")
}

// OpenConnection is kept for backward compatibility but now returns a gorm.DB
func OpenConnection() *gorm.DB {
	if DB == nil {
		InitDB()
	}
	return DB
}


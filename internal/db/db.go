package db

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"data-storage/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

func LoadConfigFromEnv() Config {
	portStr := os.Getenv("DB_PORT")
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 5432
	}

	return Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
	}
}

func InitDB(cfg Config) (*gorm.DB, error) {
	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Signal{},
		&models.SignalValue{},
	)
	if err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}

	log.Println("Database connection established and migrations completed")
	return DB, nil
}

func GetDB() *gorm.DB {
	return DB
}


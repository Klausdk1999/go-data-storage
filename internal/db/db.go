package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"data-storage/internal/models"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

type Config struct {
	Type        string // "postgres" or "sqlite"
	Host        string
	Port        int
	User        string
	Password    string
	DBName      string
	Path        string // For SQLite: file path
	SSLMode     string // For PostgreSQL: SSL mode (disable, require, verify-full)
	DatabaseURL string // Full connection string (takes priority over individual fields)
}

func LoadConfigFromEnv() Config {
	dbType := strings.ToLower(os.Getenv("DB_TYPE"))
	if dbType == "" {
		dbType = "postgres"
	}

	portStr := os.Getenv("DB_PORT")
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 5432
	}

	sslMode := os.Getenv("DB_SSLMODE")
	if sslMode == "" {
		sslMode = "require"
	}

	return Config{
		Type:        dbType,
		Host:        os.Getenv("DB_HOST"),
		Port:        port,
		User:        os.Getenv("DB_USER"),
		Password:    os.Getenv("DB_PASSWORD"),
		DBName:      os.Getenv("DB_NAME"),
		Path:        os.Getenv("DB_PATH"),
		SSLMode:     sslMode,
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
}

func InitDB(cfg Config) (*gorm.DB, error) {
	var err error
	var dsn string

	if cfg.Type == "sqlite" {
		// SQLite configuration
		if cfg.Path == "" {
			cfg.Path = "./data/storage.db" // Default path
		}

		// Ensure directory exists
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("error creating database directory: %w", err)
		}

		dsn = cfg.Path
		DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			return nil, fmt.Errorf("error connecting to SQLite database: %w", err)
		}
		log.Printf("Connected to SQLite database at: %s", cfg.Path)
	} else {
		// PostgreSQL configuration — DATABASE_URL takes priority
		if cfg.DatabaseURL != "" {
			dsn = cfg.DatabaseURL
			log.Println("Using DATABASE_URL for PostgreSQL connection")
		} else {
			if cfg.Host == "" {
				cfg.Host = "localhost"
			}
			if cfg.User == "" {
				cfg.User = "postgres"
			}
			if cfg.DBName == "" {
				cfg.DBName = "iotdb"
			}

			dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
		}

		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			return nil, fmt.Errorf("error connecting to PostgreSQL database: %w", err)
		}

		// Configure connection pool (important for serverless databases like Neon)
		sqlDB, err := DB.DB()
		if err != nil {
			return nil, fmt.Errorf("error getting underlying sql.DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)

		if cfg.DatabaseURL != "" {
			log.Println("Connected to PostgreSQL via DATABASE_URL")
		} else {
			log.Printf("Connected to PostgreSQL database: %s@%s:%d/%s", cfg.User, cfg.Host, cfg.Port, cfg.DBName)
		}
	}

	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&models.User{},
		&models.Device{},
		&models.Signal{},
		&models.SignalValue{},
		&models.Product{},
		&models.RawMaterial{},
		&models.BillOfMaterials{},
		&models.Customer{},
		&models.ProductionOrder{},
		&models.StockMovement{},
		&models.Service{},
		&models.TimeEntry{},
	)
	if err != nil {
		return nil, fmt.Errorf("error migrating database: %w", err)
	}

	log.Println("Database migrations completed")
	return DB, nil
}

func GetDB() *gorm.DB {
	return DB
}


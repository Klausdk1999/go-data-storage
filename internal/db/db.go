package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"data-storage/internal/models"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

type Config struct {
	Type     string // "postgres" or "sqlite"
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Path     string // For SQLite: file path
}

func LoadConfigFromEnv() Config {
	dbType := strings.ToLower(os.Getenv("DB_TYPE"))
	if dbType == "" {
		dbType = "postgres" // Default to postgres
	}

	portStr := os.Getenv("DB_PORT")
	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 5432
	}

	return Config{
		Type:     dbType,
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		Path:     os.Getenv("DB_PATH"),
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
		// PostgreSQL configuration
		if cfg.Host == "" {
			cfg.Host = "localhost"
		}
		if cfg.User == "" {
			cfg.User = "postgres"
		}
		if cfg.DBName == "" {
			cfg.DBName = "iotdb"
		}

		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)

		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			return nil, fmt.Errorf("error connecting to PostgreSQL database: %w", err)
		}
		log.Printf("Connected to PostgreSQL database: %s@%s:%d/%s", cfg.User, cfg.Host, cfg.Port, cfg.DBName)
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
		&models.ProductionOrder{},
		&models.StockMovement{},
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


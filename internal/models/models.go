package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents an authenticated user
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id,omitempty"`
	Name         string    `gorm:"not null" json:"name"`
	Email        string    `gorm:"uniqueIndex" json:"email,omitempty"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"` // Never return in JSON
	Categoria    string    `json:"categoria,omitempty"`
	Matricula    string    `json:"matricula,omitempty"`
	Rfid         string    `gorm:"uniqueIndex" json:"rfid,omitempty"`
	IsActive     bool      `gorm:"default:true" json:"is_active,omitempty"`
	Devices      []Device  `gorm:"foreignKey:UserID" json:"devices,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

// SetPassword hashes and sets the user's password
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword verifies a password against the hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// Device represents an IoT device
type Device struct {
	ID          uint      `gorm:"primaryKey" json:"id,omitempty"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description,omitempty"`
	DeviceType  string    `json:"device_type,omitempty"`
	Location    string    `json:"location,omitempty"`
	UserID      *uint     `gorm:"index" json:"user_id,omitempty"` // Optional
	User        *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	AuthToken   string    `gorm:"uniqueIndex;not null" json:"auth_token,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active,omitempty"`
	Signals     []Signal  `gorm:"foreignKey:DeviceID" json:"signals,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// Signal represents a signal configuration (input/output, analogic/digital)
type Signal struct {
	ID          uint          `gorm:"primaryKey" json:"id,omitempty"`
	DeviceID    uint          `gorm:"not null;index" json:"device_id"`
	Device      Device        `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	Name        string        `gorm:"not null" json:"name"`
	SignalType  string        `gorm:"not null;default:'analogic';check:signal_type IN ('digital','analogic')" json:"signal_type"`
	Direction   string        `gorm:"not null;default:'input';check:direction IN ('input','output')" json:"direction"`
	SensorName  string        `json:"sensor_name,omitempty"`
	Description string        `json:"description,omitempty"`
	Unit        string        `json:"unit,omitempty"`
	MinValue    *float64      `json:"min_value,omitempty"`
	MaxValue    *float64      `json:"max_value,omitempty"`
	Metadata    JSONB         `gorm:"type:jsonb" json:"metadata,omitempty"`
	IsActive    bool          `gorm:"default:true" json:"is_active,omitempty"`
	Values      []SignalValue `gorm:"foreignKey:SignalID" json:"values,omitempty"`
	CreatedAt   time.Time     `json:"created_at,omitempty"`
	UpdatedAt   time.Time     `json:"updated_at,omitempty"`
}

// SignalValue represents an actual data point/reading for a signal
type SignalValue struct {
	ID           uint      `gorm:"primaryKey" json:"id,omitempty"`
	SignalID     uint      `gorm:"not null;index" json:"signal_id"`
	Signal       Signal    `gorm:"foreignKey:SignalID" json:"signal,omitempty"`
	UserID       *uint     `gorm:"index" json:"user_id,omitempty"` // Optional, can fallback to device user
	User         *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Timestamp    time.Time `gorm:"default:CURRENT_TIMESTAMP;index" json:"timestamp"`
	Value        *float64  `json:"value,omitempty"`        // For analogic signals
	DigitalValue *bool     `json:"digital_value,omitempty"` // For digital signals
	Metadata     JSONB     `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

// JSONB is a custom type for PostgreSQL JSONB
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}


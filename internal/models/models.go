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
	Type         string    `gorm:"column:type;default:'worker';check:type IN ('admin','worker')" json:"type,omitempty"`
	Rfid         string    `gorm:"uniqueIndex" json:"rfid,omitempty"`
	IsActive     bool      `gorm:"default:true" json:"is_active,omitempty"`
	Preferences  JSONB     `gorm:"type:jsonb;default:'{}'" json:"preferences,omitempty"`
	Image        []byte    `json:"-" gorm:"type:bytea"`
	ImageType    string    `json:"-" gorm:"type:varchar(50)"`
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
	Image       []byte    `json:"-" gorm:"type:bytea"`
	ImageType   string    `json:"-" gorm:"type:varchar(50)"`
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

// Product represents a manufactured product
type Product struct {
	ID          uint              `gorm:"primaryKey" json:"id,omitempty"`
	Name        string            `gorm:"not null" json:"name"`
	SKU         string            `gorm:"uniqueIndex" json:"sku,omitempty"`
	Description string            `json:"description,omitempty"`
	Unit        string            `json:"unit,omitempty"`
	Category    string            `json:"category,omitempty"`
	IsActive    bool              `gorm:"default:true" json:"is_active"`
	Metadata    JSONB             `gorm:"type:jsonb" json:"metadata,omitempty"`
	Image       []byte            `json:"-" gorm:"type:bytea"`
	ImageType   string            `json:"-" gorm:"type:varchar(50)"`
	BOM         []BillOfMaterials `gorm:"foreignKey:ProductID" json:"bom,omitempty"`
	CreatedAt   time.Time         `json:"created_at,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at,omitempty"`
}

// RawMaterial represents raw material in stock
type RawMaterial struct {
	ID            uint      `gorm:"primaryKey" json:"id,omitempty"`
	Name          string    `gorm:"not null" json:"name"`
	SKU           string    `gorm:"uniqueIndex" json:"sku,omitempty"`
	Description   string    `json:"description,omitempty"`
	Unit          string    `json:"unit,omitempty"`
	StockQuantity float64   `gorm:"default:0" json:"stock_quantity"`
	MinStock      *float64  `json:"min_stock,omitempty"`
	Category      string    `json:"category,omitempty"`
	IsActive      bool      `gorm:"default:true" json:"is_active"`
	Metadata      JSONB     `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt     time.Time `json:"created_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

// BillOfMaterials links a product to its raw materials
type BillOfMaterials struct {
	ID            uint         `gorm:"primaryKey" json:"id,omitempty"`
	ProductID     uint         `gorm:"not null;index" json:"product_id"`
	Product       *Product     `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	RawMaterialID uint         `gorm:"not null;index" json:"raw_material_id"`
	RawMaterial   *RawMaterial `gorm:"foreignKey:RawMaterialID" json:"raw_material,omitempty"`
	Quantity      float64      `gorm:"not null" json:"quantity"`
	CreatedAt     time.Time    `json:"created_at,omitempty"`
	UpdatedAt     time.Time    `json:"updated_at,omitempty"`
}

// Customer represents a customer for production orders
type Customer struct {
	ID        uint      `gorm:"primaryKey" json:"id,omitempty"`
	Name      string    `gorm:"not null;uniqueIndex" json:"name"`
	Phone     string    `gorm:"not null;default:''" json:"phone"`
	CNPJ      *string   `json:"cnpj,omitempty"`
	Address   *string   `json:"address,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ProductionOrder represents a manufacturing order
type ProductionOrder struct {
	ID               uint       `gorm:"primaryKey" json:"id,omitempty"`
	ProductID        uint       `gorm:"not null;index" json:"product_id"`
	Product          *Product   `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	CustomerID       *uint      `gorm:"index" json:"customer_id,omitempty"`
	Customer         *Customer  `gorm:"foreignKey:CustomerID" json:"customer,omitempty"`
	Quantity         float64    `gorm:"not null" json:"quantity"`
	Status           string     `gorm:"not null;default:'planned'" json:"status"`
	Priority         int        `gorm:"default:0" json:"priority,omitempty"`
	DeviceID         *uint      `gorm:"index" json:"device_id,omitempty"`
	Device           *Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	WorkInstructions string     `json:"work_instructions,omitempty"`
	QualityNotes     string     `json:"quality_notes,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	PlannedDeliveryDate   *time.Time `json:"planned_delivery_date,omitempty"`
	Metadata         JSONB      `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt        time.Time  `json:"created_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at,omitempty"`
}

// StockMovement tracks stock changes for raw materials
type StockMovement struct {
	ID                uint         `gorm:"primaryKey" json:"id,omitempty"`
	RawMaterialID     uint         `gorm:"not null;index" json:"raw_material_id"`
	RawMaterial       *RawMaterial `gorm:"foreignKey:RawMaterialID" json:"raw_material,omitempty"`
	ProductionOrderID *uint        `gorm:"index" json:"production_order_id,omitempty"`
	MovementType      string       `gorm:"not null" json:"movement_type"`
	Quantity          float64      `gorm:"not null" json:"quantity"`
	Notes             string       `json:"notes,omitempty"`
	CreatedAt         time.Time    `json:"created_at,omitempty"`
}

// Service represents a type of work/service that can be performed
type Service struct {
	ID          uint      `gorm:"primaryKey" json:"id,omitempty"`
	Code        string    `gorm:"uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

// TimeEntry represents a time tracking entry for a user working on a production order
type TimeEntry struct {
	ID                uint             `gorm:"primaryKey" json:"id,omitempty"`
	UserID            uint             `gorm:"not null;index" json:"user_id"`
	User              *User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
	ProductionOrderID uint             `gorm:"not null;index" json:"production_order_id"`
	ProductionOrder   *ProductionOrder `gorm:"foreignKey:ProductionOrderID" json:"production_order,omitempty"`
	ServiceID         uint             `gorm:"not null;index" json:"service_id"`
	Service           *Service         `gorm:"foreignKey:ServiceID" json:"service,omitempty"`
	Day               string           `gorm:"not null" json:"day"`
	StartTime         string           `gorm:"not null" json:"start_time"`
	EndTime           string           `gorm:"not null" json:"end_time"`
	Observations      string           `json:"observations,omitempty"`
	CreatedAt         time.Time        `json:"created_at,omitempty"`
	UpdatedAt         time.Time        `json:"updated_at,omitempty"`
}


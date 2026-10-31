package sql

import (
	"time"

	"github.com/google/uuid"
)

type unitDBO struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	Code       string
	Name       string
	BaseUnitID *uuid.UUID
	BaseSize   *float64
	SoldWhole  bool
	Position   int
}

func (unitDBO) TableName() string { return "units" }

type categoryDBO struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	OwnerID   string
	Name      string
	Emoji     string
	Source    string
	SystemKey *string
	Hidden    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (categoryDBO) TableName() string { return "grocery_categories" }

type itemDBO struct {
	ID         uuid.UUID `gorm:"primaryKey"`
	OwnerID    string
	Name       string
	Emoji      string
	CategoryID uuid.UUID
	Source     string
	SystemKey  *string
	Hidden     bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (itemDBO) TableName() string { return "grocery_items" }

const (
	purposeBuy  = "buy"
	purposeCook = "cook"
)

type itemUnitDBO struct {
	ItemID   uuid.UUID `gorm:"primaryKey"`
	Purpose  string    `gorm:"primaryKey"`
	UnitID   uuid.UUID `gorm:"primaryKey"`
	Position int
}

func (itemUnitDBO) TableName() string { return "grocery_item_units" }

type itemConversionDBO struct {
	ItemID     uuid.UUID `gorm:"primaryKey"`
	FromUnitID uuid.UUID `gorm:"primaryKey"`
	ToUnitID   uuid.UUID `gorm:"primaryKey"`
	Factor     float64
	Position   int
}

func (itemConversionDBO) TableName() string { return "grocery_item_conversions" }

type starterDataCopyDBO struct {
	OwnerID  string `gorm:"primaryKey"`
	CopiedAt time.Time
}

func (starterDataCopyDBO) TableName() string { return "starter_data_copies" }

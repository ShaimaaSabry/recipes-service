package sql

import "time"

type recipeDBO struct {
	OwnerID   string `gorm:"primaryKey"`
	ID        string `gorm:"primaryKey"`
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (recipeDBO) TableName() string { return "recipes" }

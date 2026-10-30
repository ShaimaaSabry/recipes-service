package sql

import (
	"time"

	"github.com/google/uuid"
)

type userDBO struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	Email     string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (userDBO) TableName() string { return "users" }

type refreshTokenDBO struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	UserID    uuid.UUID
	FamilyID  uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (refreshTokenDBO) TableName() string { return "refresh_tokens" }

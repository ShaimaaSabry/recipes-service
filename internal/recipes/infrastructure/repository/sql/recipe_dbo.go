package sql

import (
	"time"

	"github.com/google/uuid"
)

type recipeDBO struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	OwnerID   string
	Name      string
	Servings  int
	PhotoKey  *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (recipeDBO) TableName() string { return "recipes" }

type ingredientDBO struct {
	RecipeID uuid.UUID `gorm:"primaryKey"`
	Position int       `gorm:"primaryKey"`
	OwnerID  string
	ItemID   uuid.UUID
	UnitID   uuid.UUID
	Quantity float64
}

func (ingredientDBO) TableName() string { return "recipe_ingredients" }

type stepDBO struct {
	RecipeID uuid.UUID `gorm:"primaryKey"`
	Position int       `gorm:"primaryKey"`
	Text     string
}

func (stepDBO) TableName() string { return "recipe_steps" }

type plannedMealDBO struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	OwnerID   string
	RecipeID  uuid.UUID
	Servings  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (plannedMealDBO) TableName() string { return "planned_meals" }

type groceryLineDBO struct {
	ID        uuid.UUID `gorm:"primaryKey"`
	OwnerID   string
	ItemID    uuid.UUID
	UnitID    uuid.UUID
	Quantity  float64
	Checked   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (groceryLineDBO) TableName() string { return "grocery_list_lines" }

type appliedActionDBO struct {
	OwnerID   string    `gorm:"primaryKey"`
	ActionID  uuid.UUID `gorm:"primaryKey"`
	AppliedAt time.Time
}

func (appliedActionDBO) TableName() string { return "applied_actions" }

type localImportDBO struct {
	OwnerID    string    `gorm:"primaryKey"`
	ImportID   uuid.UUID `gorm:"primaryKey"`
	Result     *string
	ImportedAt time.Time
}

func (localImportDBO) TableName() string { return "local_imports" }

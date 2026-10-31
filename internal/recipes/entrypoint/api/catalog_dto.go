package api

import (
	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/application/getcatalog"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type categoryRequest struct {
	Name  string `json:"name" binding:"required" example:"Frozen"`
	Emoji string `json:"emoji" binding:"required" example:"🧊"`
}

type hiddenRequest struct {
	// Hidden leaves the record out of the app's pickers; false shows it again.
	Hidden *bool `json:"hidden" binding:"required" example:"true"`
}

type conversionDTO struct {
	FromUnitID string  `json:"fromUnitId" binding:"required,uuid" example:"01a19182-b211-7207-a87a-4bec8f271f32"`
	ToUnitID   string  `json:"toUnitId" binding:"required,uuid" example:"01a19182-b211-7226-8fd7-e7f5ebfa367e"`
	Factor     float64 `json:"factor" binding:"required,gt=0" example:"10"`
}

type createItemRequest struct {
	Name        string          `json:"name" binding:"required" example:"Black garlic"`
	Emoji       string          `json:"emoji" binding:"required" example:"🧄"`
	CategoryID  string          `json:"categoryId" binding:"required,uuid"`
	BuyUnitIDs  []string        `json:"buyUnitIds" binding:"required,min=1,dive,uuid"`
	CookUnitIDs []string        `json:"cookUnitIds" binding:"dive,uuid"`
	Conversions []conversionDTO `json:"conversions" binding:"dive"`
}

type editItemRequest struct {
	Name        string   `json:"name" binding:"required" example:"Black garlic"`
	Emoji       string   `json:"emoji" binding:"required" example:"🧄"`
	CategoryID  string   `json:"categoryId" binding:"required,uuid"`
	BuyUnitIDs  []string `json:"buyUnitIds" binding:"required,min=1,dive,uuid"`
	CookUnitIDs []string `json:"cookUnitIds" binding:"dive,uuid"`
	// Conversions replace the item's. Leave it out to keep the saved ones (minus any naming a unit
	// the item no longer has).
	Conversions *[]conversionDTO `json:"conversions" binding:"omitempty,dive"`
}

type unitDTO struct {
	ID   string `json:"id"`
	Code string `json:"code" example:"cup"`
	Name string `json:"name" example:"cup"`
	// BaseUnitID and BaseSize: one of this unit is BaseSize of BaseUnitID (1 cup = 240 ml). Units
	// with the same base convert into each other. Both null for units whose size depends on the
	// item (pack, head…).
	BaseUnitID *string  `json:"baseUnitId"`
	BaseSize   *float64 `json:"baseSize" example:"240"`
	// SoldWhole: the grocery list rounds amounts in this unit up to whole numbers.
	SoldWhole bool `json:"soldWhole"`
}

type categoryDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name" example:"Produce"`
	Emoji  string `json:"emoji" example:"🥕"`
	Source string `json:"source" enums:"system,user"`
	Hidden bool   `json:"hidden"`
}

type itemDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name" example:"Garlic"`
	Emoji       string          `json:"emoji" example:"🧄"`
	CategoryID  string          `json:"categoryId"`
	BuyUnitIDs  []string        `json:"buyUnitIds"`
	CookUnitIDs []string        `json:"cookUnitIds"`
	Conversions []conversionDTO `json:"conversions"`
	Source      string          `json:"source" enums:"system,user"`
	Hidden      bool            `json:"hidden"`
}

type catalogDTO struct {
	Units      []unitDTO     `json:"units"`
	Categories []categoryDTO `json:"categories"`
	Items      []itemDTO     `json:"items"`
}

func toCatalogDTO(catalog getcatalog.Catalog) catalogDTO {
	dto := catalogDTO{
		Units:      make([]unitDTO, 0, len(catalog.Units)),
		Categories: make([]categoryDTO, 0, len(catalog.Categories)),
		Items:      make([]itemDTO, 0, len(catalog.Items)),
	}
	for _, u := range catalog.Units {
		unit := unitDTO{ID: u.ID().String(), Code: u.Code(), Name: u.Name(), SoldWhole: u.SoldWhole()}
		if base, size, ok := u.Base(); ok {
			id := base.String()
			unit.BaseUnitID, unit.BaseSize = &id, &size
		}
		dto.Units = append(dto.Units, unit)
	}
	for _, c := range catalog.Categories {
		dto.Categories = append(dto.Categories, toCategoryDTO(c))
	}
	for _, i := range catalog.Items {
		dto.Items = append(dto.Items, toItemDTO(i))
	}
	return dto
}

func toCategoryDTO(c *model.Category) categoryDTO {
	return categoryDTO{ID: c.ID().String(), Name: c.Name(), Emoji: c.Emoji(), Source: string(c.Source()), Hidden: c.Hidden()}
}

func toItemDTO(i *model.GroceryItem) itemDTO {
	conversions := make([]conversionDTO, 0, len(i.Conversions()))
	for _, c := range i.Conversions() {
		conversions = append(conversions, conversionDTO{FromUnitID: c.From.String(), ToUnitID: c.To.String(), Factor: c.Factor})
	}
	return itemDTO{
		ID: i.ID().String(), Name: i.Name(), Emoji: i.Emoji(), CategoryID: i.CategoryID().String(),
		BuyUnitIDs: idStrings(i.BuyUnitIDs()), CookUnitIDs: idStrings(i.CookUnitIDs()), Conversions: conversions,
		Source: string(i.Source()), Hidden: i.Hidden(),
	}
}

func idStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// The binding tags have already checked every ID is a UUID, so these can't fail.

func toConversion(dto conversionDTO) model.Conversion {
	return model.Conversion{From: uuid.MustParse(dto.FromUnitID), To: uuid.MustParse(dto.ToUnitID), Factor: dto.Factor}
}

func toConversions(dtos []conversionDTO) []model.Conversion {
	conversions := make([]model.Conversion, 0, len(dtos))
	for _, dto := range dtos {
		conversions = append(conversions, toConversion(dto))
	}
	return conversions
}

func toItemDetails(name, emoji, categoryID string, buy, cook []string) model.ItemDetails {
	return model.ItemDetails{
		Name: name, Emoji: emoji, CategoryID: uuid.MustParse(categoryID),
		BuyUnitIDs: parseIDs(buy), CookUnitIDs: parseIDs(cook),
	}
}

func parseIDs(ids []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, uuid.MustParse(id))
	}
	return out
}

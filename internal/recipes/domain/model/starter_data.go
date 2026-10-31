package model

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// StarterData is the categories and grocery items every account starts with. Entries refer to
// each other by key and to units by code, never by ID: each account gets copies with IDs of its
// own, which remember their entry's key (system_key).
type StarterData struct {
	Categories []StarterCategory
	Items      []StarterItem
}

type StarterCategory struct {
	Key   string
	Name  string
	Emoji string
}

type StarterItem struct {
	Key      string
	Name     string
	Emoji    string
	Category string // a StarterCategory's key
	Buy      []string
	Cook     []string
	// Conversions are [from, to] unit codes with a factor: 1 from = factor to.
	Conversions []StarterConversion
}

type StarterConversion struct {
	From   string
	To     string
	Factor float64
}

// CopyFor builds owner's copies of the starter categories and items, each with an ID from newID.
func (s StarterData) CopyFor(owner owner.ID, units Units, newID func() (uuid.UUID, error)) ([]*Category, []*GroceryItem, error) {
	unitByCode := map[string]uuid.UUID{}
	for _, unit := range units.All() {
		unitByCode[unit.Code()] = unit.ID()
	}
	unitIDs := func(item string, codes []string) ([]uuid.UUID, error) {
		ids := make([]uuid.UUID, 0, len(codes))
		for _, code := range codes {
			id, ok := unitByCode[code]
			if !ok {
				return nil, fmt.Errorf("starter item %q: no unit %q", item, code)
			}
			ids = append(ids, id)
		}
		return ids, nil
	}

	categoryByKey := map[string]uuid.UUID{}
	categories := make([]*Category, 0, len(s.Categories))
	for _, entry := range s.Categories {
		id, err := newID()
		if err != nil {
			return nil, nil, err
		}
		category, err := NewStarterCategory(id, owner, entry.Key, entry.Name, entry.Emoji)
		if err != nil {
			return nil, nil, fmt.Errorf("starter category %q: %w", entry.Key, err)
		}
		categoryByKey[entry.Key] = id
		categories = append(categories, category)
	}

	items := make([]*GroceryItem, 0, len(s.Items))
	for _, entry := range s.Items {
		categoryID, ok := categoryByKey[entry.Category]
		if !ok {
			return nil, nil, fmt.Errorf("starter item %q: no category %q", entry.Key, entry.Category)
		}
		buy, err := unitIDs(entry.Key, entry.Buy)
		if err != nil {
			return nil, nil, err
		}
		cook, err := unitIDs(entry.Key, entry.Cook)
		if err != nil {
			return nil, nil, err
		}
		conversions := make([]Conversion, 0, len(entry.Conversions))
		for _, c := range entry.Conversions {
			ids, err := unitIDs(entry.Key, []string{c.From, c.To})
			if err != nil {
				return nil, nil, err
			}
			conversions = append(conversions, Conversion{From: ids[0], To: ids[1], Factor: c.Factor})
		}
		id, err := newID()
		if err != nil {
			return nil, nil, err
		}
		details := ItemDetails{Name: entry.Name, Emoji: entry.Emoji, CategoryID: categoryID, BuyUnitIDs: buy, CookUnitIDs: cook}
		item, err := NewStarterGroceryItem(id, owner, entry.Key, details, conversions, units)
		if err != nil {
			return nil, nil, fmt.Errorf("starter item %q: %w", entry.Key, err)
		}
		items = append(items, item)
	}
	return categories, items, nil
}

package model

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// LocalData is what the app stored on the phone before accounts existed (its data version 4),
// sent once so nothing is lost. Records refer to each other by the app's own string IDs. Starter
// records have the IDs the app's seed gave them: "cat-produce", "item-tomatoes", "unit-kg", the
// same keys the service's starter data uses.
type LocalData struct {
	Categories  []LocalCategory
	Units       []LocalUnit
	Items       []LocalItem
	Recipes     []LocalRecipe
	Meals       []LocalMeal
	GroceryList []LocalLine
}

type LocalCategory struct {
	ID     string
	Name   string
	Emoji  string
	Source Source
	Hidden bool
}

type LocalUnit struct {
	ID   string
	Name string
}

type LocalItem struct {
	ID          string
	Name        string
	Emoji       string
	CategoryID  string
	BuyUnitIDs  []string
	CookUnitIDs []string
	Conversions []LocalConversion
	Source      Source
	Hidden      bool
}

type LocalConversion struct {
	FromUnitID string
	ToUnitID   string
	Factor     float64
}

type LocalRecipe struct {
	ID          string
	Name        string
	Servings    int
	Ingredients []LocalIngredient
	Steps       []string
}

type LocalIngredient struct {
	ItemID   string
	UnitID   string
	Quantity float64
}

// LocalMeal is a planned meal. Cooked ones aren't imported: the menu only holds meals to come.
type LocalMeal struct {
	RecipeID string
	Servings int
	Cooked   bool
}

type LocalLine struct {
	ItemID   string
	UnitID   string
	Quantity float64
	Checked  bool
}

// ImportState is what the account already has when the import runs: its starter copies, and
// anything another phone imported or added first.
type ImportState struct {
	Units      Units
	Categories []*Category
	Items      []*GroceryItem
	// Usage is where each item is used, so the import never takes away a unit that's in use.
	Usage map[uuid.UUID]ItemUsage
	Lines []*GroceryLine
}

// ImportPlan is everything the import saves.
type ImportPlan struct {
	NewCategories     []*Category
	ChangedCategories []*Category
	NewItems          []*GroceryItem
	ChangedItems      []*GroceryItem
	Recipes           []*Recipe
	Meals             []*PlannedMeal
	// Lines are new lines and changed ones, saved by ID.
	Lines  []*GroceryLine
	Result ImportResult
}

// ImportResult is what the app needs after the import.
type ImportResult struct {
	// RecipeIDs maps each local recipe ID to the recipe's new ID, for uploading its photo.
	RecipeIDs map[string]uuid.UUID `json:"recipeIds"`
	// Notes say what the import had to change or leave out, in words for the user.
	Notes []string `json:"notes"`
}

const (
	importedRecipeName   = "Untitled recipe"
	importedItemName     = "Unnamed item"
	importedCategoryName = "Other"
	importedEmoji        = "🛒"
	pieceCode            = "piece"
)

// PlanImport works out how local fits into the account. It's forgiving: data the service would
// refuse from a form (a unit it doesn't have, a conversion that clashes, an ingredient without
// an amount) is fixed or left out, with a note, rather than failing the whole import. It fails
// only when the result still breaks a rule, which would be a bug.
//
//   - Starter categories and items are matched to the account's copies by key, and take on the
//     local name, emoji, units, conversions and hidden flag. Others are matched by name, or created.
//   - Units the service doesn't have become "piece".
//   - Units are added to an item where a recipe or the list needs them, and never taken away
//     while something uses them.
//   - Recipes are always created. Meals not yet cooked are planned. Grocery lines join the item's
//     line if it has one.
func PlanImport(owner owner.ID, local LocalData, state ImportState, newID func() uuid.UUID) (*ImportPlan, error) {
	p := &importPlanner{
		owner: owner, units: state.Units, state: state, newID: newID,
		plan:          &ImportPlan{Result: ImportResult{RecipeIDs: map[string]uuid.UUID{}, Notes: []string{}}},
		unitIDs:       map[string]uuid.UUID{},
		categoryByID:  map[string]*Category{},
		itemByLocalID: map[string]*itemDraft{},
	}
	if err := p.mapUnits(local.Units); err != nil {
		return nil, err
	}
	if err := p.mapCategories(local.Categories); err != nil {
		return nil, err
	}
	p.mapItems(local.Items)
	recipes := p.draftRecipes(local.Recipes)
	lines := p.draftLines(local.GroceryList)
	items, err := p.finishItems()
	if err != nil {
		return nil, err
	}
	if err := p.planRecipes(recipes, items); err != nil {
		return nil, err
	}
	if err := p.planMeals(local.Meals); err != nil {
		return nil, err
	}
	if err := p.planLines(lines); err != nil {
		return nil, err
	}
	return p.plan, nil
}

type importPlanner struct {
	owner owner.ID
	units Units
	state ImportState
	newID func() uuid.UUID
	plan  *ImportPlan

	piece   uuid.UUID
	unitIDs map[string]uuid.UUID
	// unknownUnits are local units that became piece, by local ID, for notes.
	unknownUnits map[string]string

	categoryByID map[string]*Category
	categories   []*Category
	fallback     *Category

	itemByLocalID map[string]*itemDraft
	items         []*itemDraft
}

// itemDraft is an item as the import will save it, built up before it's validated.
type itemDraft struct {
	existing    *GroceryItem // nil for a new item
	id          uuid.UUID
	systemKey   string
	details     ItemDetails
	conversions []Conversion
	hidden      bool
	changed     bool
}

func (p *importPlanner) note(format string, args ...any) {
	p.plan.Result.Notes = append(p.plan.Result.Notes, fmt.Sprintf(format, args...))
}

// ---------------------------------------------------------------------------
// Units
// ---------------------------------------------------------------------------

// mapUnits matches each local unit to a unit by its starter ID ("unit-kg") or its name, or to
// piece.
func (p *importPlanner) mapUnits(local []LocalUnit) error {
	byCode := map[string]uuid.UUID{}
	for _, unit := range p.units.All() {
		byCode[strings.ToLower(unit.Code())] = unit.ID()
		if _, taken := byCode[strings.ToLower(unit.Name())]; !taken {
			byCode[strings.ToLower(unit.Name())] = unit.ID()
		}
	}
	piece, ok := byCode[pieceCode]
	if !ok {
		return fmt.Errorf("import: no %q unit", pieceCode)
	}
	p.piece = piece
	p.unknownUnits = map[string]string{}
	for _, unit := range local {
		if id, ok := byCode[strings.ToLower(strings.TrimPrefix(unit.ID, "unit-"))]; ok && strings.HasPrefix(unit.ID, "unit-") {
			p.unitIDs[unit.ID] = id
			continue
		}
		if id, ok := byCode[strings.ToLower(normalizeName(unit.Name))]; ok {
			p.unitIDs[unit.ID] = id
			continue
		}
		p.unitIDs[unit.ID] = piece
		p.unknownUnits[unit.ID] = normalizeName(unit.Name)
	}
	return nil
}

// unit is the unit a local unit ID maps to. An ID the local data doesn't list is a starter one
// ("unit-kg") or else piece.
func (p *importPlanner) unit(localID string) uuid.UUID {
	if id, ok := p.unitIDs[localID]; ok {
		return id
	}
	if code, ok := strings.CutPrefix(localID, "unit-"); ok {
		for _, unit := range p.units.All() {
			if strings.EqualFold(unit.Code(), code) {
				p.unitIDs[localID] = unit.ID()
				return unit.ID()
			}
		}
	}
	p.unitIDs[localID] = p.piece
	p.unknownUnits[localID] = localID
	return p.piece
}

func (p *importPlanner) unitList(localIDs []string) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(localIDs))
	for _, localID := range localIDs {
		if id := p.unit(localID); !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

func (p *importPlanner) mapCategories(local []LocalCategory) error {
	p.categories = append(p.categories, p.state.Categories...)
	changed := map[uuid.UUID]bool{}
	for _, c := range local {
		name, emoji := orDefault(tidyName(c.Name), importedCategoryName), tidyEmoji(c.Emoji)
		if existing := p.starterCategory(c); existing != nil {
			if !SameName(existing.Name(), name) && p.findCategory(name, existing) != nil {
				p.note("The category “%s” kept its name: you already have a category called “%s”.", existing.Name(), name)
				name = existing.Name()
			}
			if name != existing.Name() || emoji != existing.Emoji() || c.Hidden != existing.Hidden() {
				if err := existing.Edit(name, emoji); err != nil {
					return fmt.Errorf("import category %q: %w", c.ID, err)
				}
				existing.SetHidden(c.Hidden)
				changed[existing.ID()] = true
			}
			p.categoryByID[c.ID] = existing
			continue
		}
		if same := p.findCategory(name, nil); same != nil {
			p.categoryByID[c.ID] = same
			continue
		}
		category, err := NewCategory(p.newID(), p.owner, name, emoji)
		if err != nil {
			return fmt.Errorf("import category %q: %w", c.ID, err)
		}
		category.SetHidden(c.Hidden)
		p.categories = append(p.categories, category)
		p.plan.NewCategories = append(p.plan.NewCategories, category)
		p.categoryByID[c.ID] = category
	}
	for _, category := range p.state.Categories {
		if changed[category.ID()] {
			p.plan.ChangedCategories = append(p.plan.ChangedCategories, category)
		}
	}
	return nil
}

// starterCategory is the account's copy of a local starter category, or nil.
func (p *importPlanner) starterCategory(c LocalCategory) *Category {
	key, ok := strings.CutPrefix(c.ID, "cat-")
	if c.Source != SourceSystem || !ok {
		return nil
	}
	for _, category := range p.state.Categories {
		if category.SystemKey() == key {
			return category
		}
	}
	return nil
}

// findCategory is the category called name, other than except, or nil.
func (p *importPlanner) findCategory(name string, except *Category) *Category {
	for _, category := range p.categories {
		if category != except && SameName(category.Name(), name) {
			return category
		}
	}
	return nil
}

// category is the category for a local category ID. An item whose category is missing goes in
// the first one, by name.
func (p *importPlanner) category(localID string) uuid.UUID {
	if category, ok := p.categoryByID[localID]; ok {
		return category.ID()
	}
	if p.fallback == nil {
		for _, category := range p.categories {
			if p.fallback == nil || strings.ToLower(category.Name()) < strings.ToLower(p.fallback.Name()) {
				p.fallback = category
			}
		}
	}
	if p.fallback == nil {
		category, _ := NewCategory(p.newID(), p.owner, importedCategoryName, importedEmoji)
		p.categories = append(p.categories, category)
		p.plan.NewCategories = append(p.plan.NewCategories, category)
		p.fallback = category
	}
	return p.fallback.ID()
}

// ---------------------------------------------------------------------------
// Grocery items
// ---------------------------------------------------------------------------

func (p *importPlanner) mapItems(local []LocalItem) {
	for _, existing := range p.state.Items {
		draft := &itemDraft{
			existing: existing, id: existing.ID(), systemKey: existing.SystemKey(),
			details: existing.details, conversions: existing.Conversions(), hidden: existing.Hidden(),
		}
		draft.details.BuyUnitIDs = slices.Clone(draft.details.BuyUnitIDs)
		draft.details.CookUnitIDs = slices.Clone(draft.details.CookUnitIDs)
		p.items = append(p.items, draft)
	}

	for _, i := range local {
		name := orDefault(tidyName(i.Name), importedItemName)
		details := ItemDetails{
			Name: name, Emoji: tidyEmoji(i.Emoji), CategoryID: p.category(i.CategoryID),
			BuyUnitIDs: p.unitList(i.BuyUnitIDs), CookUnitIDs: p.unitList(i.CookUnitIDs),
		}
		if len(details.BuyUnitIDs) == 0 {
			details.BuyUnitIDs = []uuid.UUID{p.piece}
		}
		conversions := p.conversions(i.Conversions)

		if draft := p.starterItem(i); draft != nil {
			if !SameName(draft.details.Name, name) && p.findItem(name, draft) != nil {
				p.note("The item “%s” kept its name: you already have an item called “%s”.", draft.details.Name, name)
				details.Name = draft.details.Name
			}
			// The local item's conversions come first; the account's own (say, added on another
			// phone) are kept where they don't clash.
			draft.conversions = p.fitConversions(details.Name, append(conversions, draft.conversions...), len(conversions))
			draft.details, draft.hidden, draft.changed = details, i.Hidden, true
			p.keepUnitsInUse(draft)
			p.itemByLocalID[i.ID] = draft
			continue
		}
		if draft := p.findItem(name, nil); draft != nil {
			// The same item, made on another phone or already imported: it keeps what it has,
			// and gains the local conversions that fit.
			before := len(draft.conversions)
			all := append(slices.Clone(draft.conversions), conversions...)
			draft.conversions = p.fitConversions(draft.details.Name, all, len(all))
			draft.changed = draft.changed || len(draft.conversions) != before
			p.itemByLocalID[i.ID] = draft
			continue
		}
		draft := &itemDraft{
			id: p.newID(), details: details,
			conversions: p.fitConversions(name, conversions, len(conversions)), hidden: i.Hidden, changed: true,
		}
		p.items = append(p.items, draft)
		p.itemByLocalID[i.ID] = draft
	}
}

// starterItem is the draft of the account's copy of a local starter item, or nil.
func (p *importPlanner) starterItem(i LocalItem) *itemDraft {
	key, ok := strings.CutPrefix(i.ID, "item-")
	if i.Source != SourceSystem || !ok {
		return nil
	}
	for _, draft := range p.items {
		if draft.existing != nil && draft.systemKey == key {
			return draft
		}
	}
	return nil
}

// findItem is the draft of the item called name, other than except, or nil.
func (p *importPlanner) findItem(name string, except *itemDraft) *itemDraft {
	for _, draft := range p.items {
		if draft != except && SameName(draft.details.Name, name) {
			return draft
		}
	}
	return nil
}

func (p *importPlanner) conversions(local []LocalConversion) []Conversion {
	conversions := make([]Conversion, 0, len(local))
	for _, c := range local {
		conversions = append(conversions, Conversion{From: p.unit(c.FromUnitID), To: p.unit(c.ToUnitID), Factor: c.Factor})
	}
	return conversions
}

// fitConversions keeps each conversion, in order, that fits with the ones kept before it: one
// that already follows from them or from the units, or that's broken, is left out. Leaving out
// one of the first noted candidates (the local ones) gets a note when it disagrees with the rest.
func (p *importPlanner) fitConversions(itemName string, candidates []Conversion, noted int) []Conversion {
	kept := make([]Conversion, 0, len(candidates))
	for n, c := range candidates {
		if slices.Contains(kept, c) {
			continue
		}
		if c.From == c.To {
			continue // both units became piece
		}
		if err := validateConversions(p.units, append(slices.Clone(kept), c)); err != nil {
			if factor, follows := Factor(p.units, kept, c.From, c.To); n >= noted || follows && sameFactor(factor, c.Factor) {
				continue // the account's own rule, or the same rule known another way
			}
			p.note("%s: left out the conversion 1 %s = %v %s, which clashes with the item's others.",
				itemName, p.units.name(c.From), c.Factor, p.units.name(c.To))
			continue
		}
		kept = append(kept, c)
	}
	return kept
}

// keepUnitsInUse adds back any unit the account's recipes or list use the item in.
func (p *importPlanner) keepUnitsInUse(draft *itemDraft) {
	usage := p.state.Usage[draft.id]
	for _, unitID := range usage.RecipeUnitIDs {
		draft.allowInRecipes(unitID)
	}
	for _, unitID := range usage.ListUnitIDs {
		draft.allowOnList(unitID)
	}
}

// allowInRecipes makes unitID one recipes may measure the item in. An item nobody cooked with
// keeps its buy units usable in recipes too.
func (d *itemDraft) allowInRecipes(unitID uuid.UUID) {
	switch {
	case len(d.details.CookUnitIDs) == 0 && slices.Contains(d.details.BuyUnitIDs, unitID):
	case len(d.details.CookUnitIDs) == 0:
		d.details.CookUnitIDs = append(slices.Clone(d.details.BuyUnitIDs), unitID)
		d.changed = true
	case !slices.Contains(d.details.CookUnitIDs, unitID):
		d.details.CookUnitIDs = append(d.details.CookUnitIDs, unitID)
		d.changed = true
	}
}

// allowOnList makes unitID one the item is bought in.
func (d *itemDraft) allowOnList(unitID uuid.UUID) {
	if !slices.Contains(d.details.BuyUnitIDs, unitID) {
		d.details.BuyUnitIDs = append(d.details.BuyUnitIDs, unitID)
		d.changed = true
	}
}

// finishItems validates the drafts into items, and returns every item by ID.
func (p *importPlanner) finishItems() (map[uuid.UUID]*GroceryItem, error) {
	unknown := make([]string, 0, len(p.unknownUnits))
	for localID, name := range p.unknownUnits {
		if name == localID {
			name = strings.TrimPrefix(localID, "unit-")
		}
		unknown = append(unknown, name)
	}
	slices.Sort(unknown)
	for _, name := range unknown {
		p.note("The unit “%s” isn't one the app has any more, so amounts in it are now in pieces.", name)
	}

	items := make(map[uuid.UUID]*GroceryItem, len(p.items))
	for _, draft := range p.items {
		if draft.existing != nil && !draft.changed {
			items[draft.id] = draft.existing
			continue
		}
		item := draft.existing
		var err error
		if item != nil {
			err = item.Edit(draft.details, draft.conversions, p.units)
		} else {
			item, err = NewGroceryItem(draft.id, p.owner, draft.details, draft.conversions, p.units)
		}
		if err != nil {
			return nil, fmt.Errorf("import item %q: %w", draft.details.Name, err)
		}
		item.SetHidden(draft.hidden)
		items[draft.id] = item
		if draft.existing != nil {
			p.plan.ChangedItems = append(p.plan.ChangedItems, item)
		} else {
			p.plan.NewItems = append(p.plan.NewItems, item)
		}
	}
	return items, nil
}

// ---------------------------------------------------------------------------
// Recipes, meals and the grocery list
// ---------------------------------------------------------------------------

type recipeDraft struct {
	localID     string
	name        string
	servings    int
	ingredients []ingredientDraft
	steps       []string
}

type ingredientDraft struct {
	item     *itemDraft
	unitID   uuid.UUID
	quantity float64
}

// draftRecipes maps the recipes' ingredients, and makes their units ones the items are cooked in.
func (p *importPlanner) draftRecipes(local []LocalRecipe) []recipeDraft {
	drafts := make([]recipeDraft, 0, len(local))
	for _, r := range local {
		name := orDefault(tidyName(r.Name), importedRecipeName)
		draft := recipeDraft{localID: r.ID, name: name, servings: clampServings(r.Servings)}
		for _, i := range r.Ingredients {
			item, ok := p.itemByLocalID[i.ItemID]
			if !ok || !(i.Quantity > 0) || math.IsInf(i.Quantity, 0) {
				p.note("%s: left out an ingredient without an item or an amount.", name)
				continue
			}
			if len(draft.ingredients) == MaxIngredients {
				p.note("%s: kept the first %d ingredients.", name, MaxIngredients)
				break
			}
			unitID := p.unit(i.UnitID)
			item.allowInRecipes(unitID)
			draft.ingredients = append(draft.ingredients, ingredientDraft{item: item, unitID: unitID, quantity: i.Quantity})
		}
		for _, step := range r.Steps {
			step = strings.TrimSpace(step)
			if step == "" {
				continue
			}
			if len(draft.steps) == MaxSteps {
				p.note("%s: kept the first %d steps.", name, MaxSteps)
				break
			}
			if utf8.RuneCountInString(step) > MaxStepLength {
				step = string([]rune(step)[:MaxStepLength])
				p.note("%s: shortened a step to %d characters.", name, MaxStepLength)
			}
			draft.steps = append(draft.steps, step)
		}
		drafts = append(drafts, draft)
	}
	return drafts
}

func (p *importPlanner) planRecipes(drafts []recipeDraft, items map[uuid.UUID]*GroceryItem) error {
	for _, draft := range drafts {
		ingredients := make([]Ingredient, 0, len(draft.ingredients))
		for _, i := range draft.ingredients {
			ingredients = append(ingredients, Ingredient{ItemID: i.item.id, UnitID: i.unitID, Quantity: RoundQuantity(i.quantity)})
		}
		details := RecipeDetails{Name: draft.name, Servings: draft.servings, Ingredients: ingredients, Steps: draft.steps}
		recipe, err := NewRecipe(p.newID(), p.owner, details, items, p.units)
		if err != nil {
			return fmt.Errorf("import recipe %q: %w", draft.name, err)
		}
		p.plan.Recipes = append(p.plan.Recipes, recipe)
		p.plan.Result.RecipeIDs[draft.localID] = recipe.ID()
	}
	return nil
}

func (p *importPlanner) planMeals(local []LocalMeal) error {
	for _, m := range local {
		recipeID, ok := p.plan.Result.RecipeIDs[m.RecipeID]
		if m.Cooked || !ok {
			continue
		}
		meal, err := NewPlannedMeal(p.newID(), p.owner, recipeID, clampServings(m.Servings))
		if err != nil {
			return fmt.Errorf("import meal: %w", err)
		}
		p.plan.Meals = append(p.plan.Meals, meal)
	}
	return nil
}

type lineDraft struct {
	item     *itemDraft
	unitID   uuid.UUID
	quantity float64
	checked  bool
}

// draftLines maps the list's lines, and makes their units ones the items are bought in.
func (p *importPlanner) draftLines(local []LocalLine) []lineDraft {
	drafts := make([]lineDraft, 0, len(local))
	for _, l := range local {
		item, ok := p.itemByLocalID[l.ItemID]
		if !ok || !(l.Quantity > 0) || math.IsInf(l.Quantity, 0) {
			p.note("Grocery list: left out a line without an item or an amount.")
			continue
		}
		unitID := p.unit(l.UnitID)
		item.allowOnList(unitID)
		drafts = append(drafts, lineDraft{item: item, unitID: unitID, quantity: l.Quantity, checked: l.Checked})
	}
	return drafts
}

// planLines puts each line on the list. An item already on it gets the amount added, unless the
// local line was already bought.
func (p *importPlanner) planLines(drafts []lineDraft) error {
	lines := map[uuid.UUID]*GroceryLine{}
	for _, line := range p.state.Lines {
		lines[line.ItemID()] = line
	}
	changed := map[uuid.UUID]bool{}
	var order []uuid.UUID
	items := map[uuid.UUID]*GroceryItem{}
	for _, item := range p.plan.NewItems {
		items[item.ID()] = item
	}
	for _, item := range p.plan.ChangedItems {
		items[item.ID()] = item
	}
	for _, item := range p.state.Items {
		if _, ok := items[item.ID()]; !ok {
			items[item.ID()] = item
		}
	}

	for _, draft := range drafts {
		item := items[draft.item.id]
		current := lines[item.ID()]
		var next *GroceryLine
		switch {
		case current == nil:
			next = GroceryLineOf(p.newID(), p.owner, item.ID(), draft.unitID, RoundQuantity(draft.quantity), draft.checked)
		case draft.checked:
			continue // already bought: nothing to add
		default:
			added, err := AddToList(current, item, p.units, draft.unitID, draft.quantity, p.newID())
			var missing *MissingConversionError
			if errors.As(err, &missing) {
				p.note("%s: left %v %s off the grocery list, as it doesn't convert to %s.",
					item.Name(), draft.quantity, p.units.name(draft.unitID), p.units.name(current.UnitID()))
				continue
			}
			if err != nil {
				return fmt.Errorf("import grocery line %q: %w", item.Name(), err)
			}
			next = added
		}
		if !changed[item.ID()] {
			order = append(order, item.ID())
		}
		lines[item.ID()], changed[item.ID()] = next, true
	}
	for _, itemID := range order {
		p.plan.Lines = append(p.plan.Lines, lines[itemID])
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tidying
// ---------------------------------------------------------------------------

func tidyName(name string) string {
	name = normalizeName(name)
	if utf8.RuneCountInString(name) > MaxNameLength {
		name = strings.TrimSpace(string([]rune(name)[:MaxNameLength]))
	}
	return name
}

func tidyEmoji(emoji string) string {
	if emoji == "" || utf8.RuneCountInString(emoji) > MaxEmojiLength {
		return importedEmoji
	}
	return emoji
}

func orDefault(name, fallback string) string {
	if name == "" {
		return fallback
	}
	return name
}

// sameFactor says whether two factors are the same rule, give or take rounding.
func sameFactor(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(a))
}

func clampServings(servings int) int {
	return min(max(servings, MinServings), MaxServings)
}

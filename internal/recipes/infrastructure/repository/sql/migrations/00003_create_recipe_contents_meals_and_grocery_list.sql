-- +goose Up

-- A recipe's ingredients, in order. The unit is one the item is cooked in (or bought in, for an
-- item nobody cooks with); the domain layer checks that. An item can't be deleted while a recipe
-- uses it: no cascade on the item.
CREATE TABLE recipe_ingredients (
    recipe_id UUID     NOT NULL,
    position  SMALLINT NOT NULL,
    owner_id  TEXT     NOT NULL,
    item_id   UUID     NOT NULL,
    unit_id   UUID     NOT NULL REFERENCES units (id),
    quantity  NUMERIC  NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (recipe_id, position),
    FOREIGN KEY (owner_id, recipe_id) REFERENCES recipes (owner_id, id) ON DELETE CASCADE,
    FOREIGN KEY (owner_id, item_id) REFERENCES grocery_items (owner_id, id)
);
CREATE INDEX recipe_ingredients_item_id ON recipe_ingredients (item_id);

CREATE TABLE recipe_steps (
    recipe_id UUID     NOT NULL REFERENCES recipes (id) ON DELETE CASCADE,
    position  SMALLINT NOT NULL,
    text      TEXT     NOT NULL,
    PRIMARY KEY (recipe_id, position)
);

-- The upcoming menu: recipes the owner means to cook, for a number of people. The app makes the
-- IDs (it plans meals offline); every other table's IDs come from the service. Deleting a recipe
-- takes its meals with it.
CREATE TABLE planned_meals (
    id         UUID        PRIMARY KEY,
    owner_id   TEXT        NOT NULL,
    recipe_id  UUID        NOT NULL,
    servings   SMALLINT    NOT NULL CHECK (servings BETWEEN 1 AND 99),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (owner_id, recipe_id) REFERENCES recipes (owner_id, id) ON DELETE CASCADE
);
CREATE INDEX planned_meals_owner_id ON planned_meals (owner_id, created_at);
CREATE INDEX planned_meals_recipe_id ON planned_meals (recipe_id);

-- The grocery list: one line per item, in one of the item's buy units. The API addresses lines by
-- item. quantity is exact (0.04 bottle after 2 tbsp of oil); it's rounded up to what you'd buy
-- only when the list is read.
CREATE TABLE grocery_list_lines (
    id         UUID        PRIMARY KEY,
    owner_id   TEXT        NOT NULL,
    item_id    UUID        NOT NULL,
    unit_id    UUID        NOT NULL REFERENCES units (id),
    quantity   NUMERIC     NOT NULL CHECK (quantity > 0),
    checked    BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, item_id),
    FOREIGN KEY (owner_id, item_id) REFERENCES grocery_items (owner_id, id)
);
CREATE INDEX grocery_list_lines_item_id ON grocery_list_lines (item_id);

-- Grocery list actions already applied, by the ID the app gave them, so an action resent after a
-- dropped connection isn't applied twice. Rows older than 30 days are cleared as new ones arrive.
CREATE TABLE applied_actions (
    owner_id   TEXT        NOT NULL,
    action_id  UUID        NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (owner_id, action_id)
);

-- +goose Down
DROP TABLE applied_actions;
DROP TABLE grocery_list_lines;
DROP TABLE planned_meals;
DROP TABLE recipe_steps;
DROP TABLE recipe_ingredients;

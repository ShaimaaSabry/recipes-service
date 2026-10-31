package recipes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/platform/database/databasetest"
	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// memoryPhotos is a photos bucket in memory. upload stands in for the app's PUT to the link.
type memoryPhotos struct {
	mu      sync.Mutex
	objects map[string]model.StoredPhoto
}

func (m *memoryPhotos) upload(key string, photo model.StoredPhoto) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = photo
}

func (m *memoryPhotos) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok
}

func (m *memoryPhotos) PresignUpload(_ context.Context, key, contentType string) (model.PhotoUpload, error) {
	return model.PhotoUpload{Key: key, URL: "https://bucket.test/" + key, Headers: map[string]string{"Content-Type": contentType}, ExpiresAt: time.Now()}, nil
}

func (m *memoryPhotos) Stat(_ context.Context, key string) (model.StoredPhoto, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	photo, ok := m.objects[key]
	return photo, ok, nil
}

func (m *memoryPhotos) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.objects, key)
	}
	return nil
}

func (m *memoryPhotos) DeleteAll(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.objects {
		if strings.HasPrefix(key, prefix) {
			delete(m.objects, key)
		}
	}
	return nil
}

func newServerWithPhotos(t *testing.T) (*gin.Engine, *Module, *memoryPhotos) {
	ctx := context.Background()
	photos := &memoryPhotos{objects: map[string]model.StoredPhoto{}}
	module, err := New(databasetest.New(t), Settings{Photos: photos, PhotosURL: "https://cdn.test"})
	require.NoError(t, err)
	require.NoError(t, module.Migrate(ctx))
	r := httpserver.New(false)
	authenticate := func(_ context.Context, token string) (owner.ID, error) { return owner.ID(token), nil }
	module.RegisterRoutes(r.Group("/v1", httpserver.RequireOwner(authenticate)))
	for _, o := range []owner.ID{"ada", "grace"} {
		require.NoError(t, module.PrepareAccount(ctx, o))
	}
	return r, module, photos
}

func TestRecipes(t *testing.T) {
	r, module, photos := newServerWithPhotos(t)
	unit, _, item, _ := catalog(t, r, "ada")
	shakshuka := fmt.Sprintf(`{"name":"Shakshuka","servings":2,
		"ingredients":[{"itemId":%q,"unitId":%q,"quantity":4},{"itemId":%q,"unitId":%q,"quantity":2}],
		"steps":["Soften the onion.","  ","Crack in the eggs."]}`,
		item["Eggs"], unit["piece"], item["Garlic"], unit["clove"])

	var recipeID string
	t.Run("creates a recipe with an ID of the service's", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/recipes", shakshuka)

		require.Equal(t, http.StatusCreated, res.status, res.body)
		created := res.json(t)
		recipeID = created["id"].(string)
		require.Equal(t, uuid.Version(7), uuid.MustParse(recipeID).Version())
		require.Equal(t, []any{"Soften the onion.", "Crack in the eggs."}, created["steps"])
		require.Nil(t, created["photoUrl"])
	})

	t.Run("scales a recipe to a number of people", func(t *testing.T) {
		res := call(r, "ada", http.MethodGet, "/v1/recipes/"+recipeID+"?servings=3", "")

		require.Equal(t, http.StatusOK, res.status, res.body)
		scaled := res.json(t)
		require.InDelta(t, 3.0, scaled["servings"], 0)
		require.InDelta(t, 6.0, scaled["ingredients"].([]any)[0].(map[string]any)["quantity"], 1e-9)

		res = call(r, "ada", http.MethodGet, "/v1/recipes/"+recipeID+"?servings=0", "")
		require.Equal(t, http.StatusBadRequest, res.status)
	})

	t.Run("refuses an ingredient in a unit the item isn't cooked in", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Soup","servings":2,"ingredients":[{"itemId":%q,"unitId":%q,"quantity":1}]}`, item["Garlic"], unit["head"])

		res := call(r, "ada", http.MethodPost, "/v1/recipes", body)

		require.Equal(t, http.StatusBadRequest, res.status)
		require.Contains(t, res.body, "Garlic isn't measured in head in recipes; use clove or g")
	})

	t.Run("refuses another account's items, and hides recipes from other accounts", func(t *testing.T) {
		require.Equal(t, http.StatusBadRequest, call(r, "grace", http.MethodPost, "/v1/recipes", shakshuka).status)
		require.Equal(t, http.StatusNotFound, call(r, "grace", http.MethodGet, "/v1/recipes/"+recipeID, "").status)
	})

	t.Run("an item a recipe uses can't lose its unit or be deleted", func(t *testing.T) {
		mine := fmt.Sprintf(`{"name":"Pickles","emoji":"🥒","categoryId":%q,"buyUnitIds":[%q],"cookUnitIds":[%q]}`,
			catalogCategory(t, r, "ada", "Pantry"), unit["jar"], unit["piece"])
		res := call(r, "ada", http.MethodPost, "/v1/items", mine)
		require.Equal(t, http.StatusCreated, res.status, res.body)
		pickles := res.json(t)["id"].(string)
		body := fmt.Sprintf(`{"name":"Burger","servings":1,"ingredients":[{"itemId":%q,"unitId":%q,"quantity":2}]}`, pickles, unit["piece"])
		require.Equal(t, http.StatusCreated, call(r, "ada", http.MethodPost, "/v1/recipes", body).status)

		edit := fmt.Sprintf(`{"name":"Pickles","emoji":"🥒","categoryId":%q,"buyUnitIds":[%q],"cookUnitIds":[%q]}`,
			catalogCategory(t, r, "ada", "Pantry"), unit["jar"], unit["g"])
		res = call(r, "ada", http.MethodPut, "/v1/items/"+pickles, edit)
		require.Equal(t, http.StatusConflict, res.status)
		require.Contains(t, res.body, "a unit you removed is still used")

		res = call(r, "ada", http.MethodDelete, "/v1/items/"+pickles, "")
		require.Equal(t, http.StatusConflict, res.status)
		require.Contains(t, res.body, "on your grocery list or in a recipe")
	})

	t.Run("uploads and attaches a photo, replacing the old one", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/recipes/"+recipeID+"/photo/upload", `{"contentType":"image/jpeg"}`)
		require.Equal(t, http.StatusOK, res.status, res.body)
		first := res.json(t)
		firstKey := first["photoKey"].(string)
		require.True(t, strings.HasPrefix(firstKey, "recipes/ada/"+recipeID+"/"), firstKey)
		require.Equal(t, "PUT", first["method"])

		res = call(r, "ada", http.MethodPut, "/v1/recipes/"+recipeID+"/photo", fmt.Sprintf(`{"photoKey":%q}`, firstKey))
		require.Equal(t, http.StatusBadRequest, res.status, "not uploaded yet")

		photos.upload(firstKey, model.StoredPhoto{Size: 1000, ContentType: "image/jpeg"})
		res = call(r, "ada", http.MethodPut, "/v1/recipes/"+recipeID+"/photo", fmt.Sprintf(`{"photoKey":%q}`, firstKey))
		require.Equal(t, http.StatusOK, res.status, res.body)
		require.Equal(t, "https://cdn.test/"+firstKey, res.json(t)["photoUrl"])

		second := call(r, "ada", http.MethodPost, "/v1/recipes/"+recipeID+"/photo/upload", `{"contentType":"image/png"}`).json(t)["photoKey"].(string)
		photos.upload(second, model.StoredPhoto{Size: 1000, ContentType: "image/png"})
		res = call(r, "ada", http.MethodPut, "/v1/recipes/"+recipeID+"/photo", fmt.Sprintf(`{"photoKey":%q}`, second))
		require.Equal(t, http.StatusOK, res.status, res.body)
		require.False(t, photos.has(firstKey), "the old photo is deleted")
	})

	t.Run("refuses a photo that's too large, and deletes it", func(t *testing.T) {
		key := call(r, "ada", http.MethodPost, "/v1/recipes/"+recipeID+"/photo/upload", `{"contentType":"image/jpeg"}`).json(t)["photoKey"].(string)
		photos.upload(key, model.StoredPhoto{Size: model.MaxPhotoSize + 1, ContentType: "image/jpeg"})

		res := call(r, "ada", http.MethodPut, "/v1/recipes/"+recipeID+"/photo", fmt.Sprintf(`{"photoKey":%q}`, key))

		require.Equal(t, http.StatusBadRequest, res.status)
		require.False(t, photos.has(key))
	})

	t.Run("refuses a photo key of another recipe or account", func(t *testing.T) {
		key := "recipes/grace/" + uuid.NewString() + "/x.jpg"
		photos.upload(key, model.StoredPhoto{Size: 1, ContentType: "image/jpeg"})

		res := call(r, "ada", http.MethodPut, "/v1/recipes/"+recipeID+"/photo", fmt.Sprintf(`{"photoKey":%q}`, key))

		require.Equal(t, http.StatusBadRequest, res.status)
	})

	t.Run("refuses a type that isn't a supported image", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/recipes/"+recipeID+"/photo/upload", `{"contentType":"application/pdf"}`)

		require.Equal(t, http.StatusBadRequest, res.status)
	})

	t.Run("deleting the account deletes its photos", func(t *testing.T) {
		require.NoError(t, module.DeleteOwnerData(context.Background(), "ada"))

		for key := range photos.objects {
			require.False(t, strings.HasPrefix(key, "recipes/ada/"), key)
		}
	})
}

func TestMenuAndGroceryList(t *testing.T) {
	r, _, _ := newServerWithPhotos(t)
	unit, _, item, _ := catalog(t, r, "ada")
	recipe := call(r, "ada", http.MethodPost, "/v1/recipes", fmt.Sprintf(
		`{"name":"Omelette","servings":1,"ingredients":[{"itemId":%q,"unitId":%q,"quantity":3}]}`, item["Eggs"], unit["piece"],
	)).json(t)["id"].(string)

	meal := uuid.NewString()
	t.Run("plans a meal under the app's ID; sending it again saves it again", func(t *testing.T) {
		body := fmt.Sprintf(`{"recipeId":%q,"servings":2}`, recipe)
		require.Equal(t, http.StatusOK, call(r, "ada", http.MethodPut, "/v1/meals/"+meal, body).status)
		res := call(r, "ada", http.MethodPut, "/v1/meals/"+meal, fmt.Sprintf(`{"recipeId":%q,"servings":4}`, recipe))
		require.Equal(t, http.StatusOK, res.status, res.body)

		var meals []map[string]any
		require.NoError(t, json.Unmarshal([]byte(call(r, "ada", http.MethodGet, "/v1/meals", "").body), &meals))
		require.Len(t, meals, 1)
		require.InDelta(t, 4.0, meals[0]["servings"], 0)
	})

	t.Run("another account can't take a meal's ID", func(t *testing.T) {
		theirs := call(r, "grace", http.MethodPost, "/v1/recipes", `{"name":"Toast","servings":1}`).json(t)["id"].(string)

		res := call(r, "grace", http.MethodPut, "/v1/meals/"+meal, fmt.Sprintf(`{"recipeId":%q,"servings":1}`, theirs))

		require.Equal(t, http.StatusConflict, res.status)
	})

	t.Run("a meal for a recipe that's gone is refused", func(t *testing.T) {
		res := call(r, "ada", http.MethodPut, "/v1/meals/"+uuid.NewString(), fmt.Sprintf(`{"recipeId":%q,"servings":1}`, uuid.NewString()))

		require.Equal(t, http.StatusBadRequest, res.status)
		require.Contains(t, res.body, "please choose one of your recipes")
	})

	type result struct {
		ID                string `json:"id"`
		Outcome           string `json:"outcome"`
		Error             string `json:"error"`
		MissingConversion *struct {
			ItemID     string `json:"itemId"`
			FromUnitID string `json:"fromUnitId"`
			ToUnitID   string `json:"toUnitId"`
		} `json:"missingConversion"`
	}
	type line struct {
		ItemID      string  `json:"itemId"`
		UnitID      string  `json:"unitId"`
		Quantity    float64 `json:"quantity"`
		BuyQuantity float64 `json:"buyQuantity"`
		Checked     bool    `json:"checked"`
	}
	send := func(t *testing.T, actions ...string) ([]result, map[string]line) {
		t.Helper()
		res := call(r, "ada", http.MethodPost, "/v1/grocery-list/actions", `{"actions":[`+strings.Join(actions, ",")+`]}`)
		require.Equal(t, http.StatusOK, res.status, res.body)
		var body struct {
			Results []result `json:"results"`
			Lines   []line   `json:"lines"`
		}
		require.NoError(t, json.Unmarshal([]byte(res.body), &body))
		lines := map[string]line{}
		for _, l := range body.Lines {
			lines[l.ItemID] = l
		}
		return body.Results, lines
	}
	action := func(id, kind, itemID, unitID string, quantity float64) string {
		return fmt.Sprintf(`{"id":%q,"type":%q,"itemId":%q,"unitId":%q,"quantity":%v}`, id, kind, itemID, unitID, quantity)
	}

	addOil := uuid.NewString()
	t.Run("adds amounts onto one line per item, converted and rounded up to buy", func(t *testing.T) {
		results, lines := send(t,
			action(addOil, "add", item["Olive oil"], unit["tbsp"], 2),
			action(uuid.NewString(), "add", item["Butter"], unit["g"], 250),
			action(uuid.NewString(), "add", item["Ground beef"], unit["g"], 700),
			action(uuid.NewString(), "add", item["Ground beef"], unit["g"], 800),
		)

		for _, res := range results {
			require.Equal(t, "applied", res.Outcome, res.Error)
		}
		require.Len(t, lines, 3)
		require.Equal(t, unit["bottle"], lines[item["Olive oil"]].UnitID)
		require.InDelta(t, 0.04, lines[item["Olive oil"]].Quantity, 1e-9)
		require.InDelta(t, 1.0, lines[item["Olive oil"]].BuyQuantity, 0)
		require.InDelta(t, 2.0, lines[item["Butter"]].BuyQuantity, 0)
		require.Equal(t, unit["kg"], lines[item["Ground beef"]].UnitID)
		require.InDelta(t, 1.5, lines[item["Ground beef"]].Quantity, 1e-9)
	})

	t.Run("a resent action is recognised, not applied twice", func(t *testing.T) {
		results, lines := send(t, action(addOil, "add", item["Olive oil"], unit["tbsp"], 2))

		require.Equal(t, "alreadyApplied", results[0].Outcome)
		require.InDelta(t, 0.04, lines[item["Olive oil"]].Quantity, 1e-9)
	})

	t.Run("an amount needing a missing conversion is rejected alone; it goes through once added", func(t *testing.T) {
		needsBox := uuid.NewString()
		results, lines := send(t,
			action(uuid.NewString(), "add", item["Eggs"], unit["piece"], 6),
			action(needsBox, "add", item["Eggs"], unit["box"], 1),
			action(uuid.NewString(), "check", item["Butter"], "", 0),
		)

		require.Equal(t, []string{"applied", "rejected", "applied"}, []string{results[0].Outcome, results[1].Outcome, results[2].Outcome})
		require.NotNil(t, results[1].MissingConversion)
		require.Equal(t, unit["box"], results[1].MissingConversion.FromUnitID)
		require.Equal(t, unit["piece"], results[1].MissingConversion.ToUnitID)
		require.Equal(t, "Eggs: box doesn't convert to piece yet. How many piece are in 1 box?", results[1].Error)
		require.True(t, lines[item["Butter"]].Checked)

		conversion := fmt.Sprintf(`{"fromUnitId":%q,"toUnitId":%q,"factor":10}`, unit["box"], unit["piece"])
		require.Equal(t, http.StatusOK, call(r, "ada", http.MethodPost, "/v1/items/"+item["Eggs"]+"/conversions", conversion).status)
		results, lines = send(t, action(needsBox, "add", item["Eggs"], unit["box"], 1))
		require.Equal(t, "applied", results[0].Outcome, results[0].Error)
		require.InDelta(t, 16.0, lines[item["Eggs"]].Quantity, 0)
	})

	t.Run("sets the amount by hand, switches the unit, removes and clears", func(t *testing.T) {
		results, lines := send(t,
			action(uuid.NewString(), "setQuantity", item["Ground beef"], "", 2),
			action(uuid.NewString(), "setUnit", item["Eggs"], unit["dozen"], 0),
			action(uuid.NewString(), "remove", item["Olive oil"], "", 0),
			action(uuid.NewString(), "remove", item["Olive oil"], "", 0),
			action(uuid.NewString(), "setQuantity", item["Olive oil"], "", 1),
		)

		require.Equal(t, []string{"applied", "applied", "applied", "applied", "rejected"},
			[]string{results[0].Outcome, results[1].Outcome, results[2].Outcome, results[3].Outcome, results[4].Outcome})
		require.InDelta(t, 2.0, lines[item["Ground beef"]].Quantity, 0)
		require.Equal(t, unit["dozen"], lines[item["Eggs"]].UnitID)
		require.InDelta(t, 16.0/12, lines[item["Eggs"]].Quantity, 1e-6)
		require.NotContains(t, lines, item["Olive oil"])

		_, lines = send(t, `{"id":"`+uuid.NewString()+`","type":"clearChecked"}`)
		require.NotContains(t, lines, item["Butter"], "butter was checked")
		require.Len(t, lines, 2)
	})

	t.Run("checks the request's shape", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/grocery-list/actions", `{"actions":[{"id":"nope","type":"teleport"}]}`)

		require.Equal(t, http.StatusBadRequest, res.status)
		require.Contains(t, res.body, `"actions[0].id":"must be a UUID"`)
	})

	t.Run("deleting a recipe takes its meals; the list stays", func(t *testing.T) {
		require.Equal(t, http.StatusNoContent, call(r, "ada", http.MethodDelete, "/v1/recipes/"+recipe, "").status)

		require.JSONEq(t, `[]`, call(r, "ada", http.MethodGet, "/v1/meals", "").body)
		require.Equal(t, http.StatusNoContent, call(r, "ada", http.MethodDelete, "/v1/meals/"+meal, "").status, "removing it again is fine")
		res := call(r, "ada", http.MethodGet, "/v1/grocery-list", "")
		require.Len(t, res.json(t)["lines"], 2)
	})
}

func catalogCategory(t *testing.T, r *gin.Engine, owner, name string) string {
	_, categories, _, _ := catalog(t, r, owner)
	require.Contains(t, categories, name)
	return categories[name]
}

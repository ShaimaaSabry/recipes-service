package recipes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

// localDataV4 is the app's seed data as the app stores it, plus what a user adds: a unit, a
// category and an item of their own, a recipe with a photo, a renamed and a hidden starter item,
// and planned meals. testdata/local_data_v4.json was made from the app's src/data/seed.ts.
func localDataV4(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/local_data_v4.json")
	require.NoError(t, err)
	return string(data)
}

func importBody(importID, data string) string {
	return fmt.Sprintf(`{"importId":%q,"data":%s}`, importID, data)
}

func TestImportLocalData(t *testing.T) {
	r, _, photos := newServerWithPhotos(t)
	importID := uuid.NewString()
	var result struct {
		Imported  bool
		RecipeIDs map[string]string
		Notes     []string
	}

	t.Run("imports the app's data into the account", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/imports", importBody(importID, localDataV4(t)))

		require.Equal(t, http.StatusOK, res.status, res.body)
		require.NoError(t, json.Unmarshal([]byte(res.body), &result))
		require.True(t, result.Imported)
		require.Len(t, result.RecipeIDs, 4)
		require.Equal(t, []string{
			"The unit “Sachet” isn't one the app has any more, so amounts in it are now in pieces.",
		}, result.Notes, "the starter data fits as it is")
	})

	unit, category, item, _ := catalog(t, r, "ada")

	t.Run("starter records take on the local changes, and the user's own are created", func(t *testing.T) {
		require.Contains(t, item, "Whole milk")
		require.NotContains(t, item, "Milk")
		require.Contains(t, category, "Baking")

		res := call(r, "ada", http.MethodGet, "/v1/catalog", "")
		var dto struct {
			Items []struct {
				Name        string
				Hidden      bool
				CategoryID  string
				BuyUnitIDs  []string
				CookUnitIDs []string
				Conversions []struct {
					FromUnitID, ToUnitID string
					Factor               float64
				}
			}
		}
		require.NoError(t, json.Unmarshal([]byte(res.body), &dto))
		for _, i := range dto.Items {
			switch i.Name {
			case "Dish soap":
				require.True(t, i.Hidden)
			case "Yeast":
				require.Equal(t, category["Baking"], i.CategoryID)
				require.Equal(t, []string{unit["piece"]}, i.BuyUnitIDs)
				require.Equal(t, []string{unit["g"]}, i.CookUnitIDs)
				require.Len(t, i.Conversions, 1)
				require.Equal(t, unit["piece"], i.Conversions[0].FromUnitID)
				require.InDelta(t, 7.0, i.Conversions[0].Factor, 0)
			}
		}
	})

	t.Run("creates the recipes, plans the meals to come and fills the list", func(t *testing.T) {
		res := call(r, "ada", http.MethodGet, "/v1/recipes/"+result.RecipeIDs["recipe-shakshuka"], "")
		require.Equal(t, http.StatusOK, res.status, res.body)
		shakshuka := res.json(t)
		require.Equal(t, "Shakshuka", shakshuka["name"])
		first := shakshuka["ingredients"].([]any)[0].(map[string]any)
		require.Equal(t, item["Eggs"], first["itemId"])
		require.Equal(t, unit["piece"], first["unitId"])

		var meals []map[string]any
		res = call(r, "ada", http.MethodGet, "/v1/meals", "")
		require.NoError(t, json.Unmarshal([]byte(res.body), &meals))
		require.Len(t, meals, 1, "the cooked meal isn't imported")
		require.Equal(t, result.RecipeIDs["recipe-shakshuka"], meals[0]["recipeId"])
		require.InDelta(t, 4.0, meals[0]["servings"], 0)

		lines := call(r, "ada", http.MethodGet, "/v1/grocery-list", "").json(t)["lines"].([]any)
		require.Len(t, lines, 7)
		byItem := map[string]map[string]any{}
		for _, l := range lines {
			line := l.(map[string]any)
			byItem[line["itemId"].(string)] = line
		}
		require.Equal(t, unit["l"], byItem[item["Whole milk"]]["unitId"])
		require.Equal(t, true, byItem[item["Bread"]]["checked"])
		require.InDelta(t, 2.0, byItem[item["Yeast"]]["quantity"], 0)
	})

	t.Run("a resend returns the first import's result and imports nothing", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/imports", importBody(importID, localDataV4(t)))

		require.Equal(t, http.StatusOK, res.status, res.body)
		var again struct {
			Imported  bool
			RecipeIDs map[string]string
		}
		require.NoError(t, json.Unmarshal([]byte(res.body), &again))
		require.False(t, again.Imported)
		require.Equal(t, result.RecipeIDs, again.RecipeIDs)
		var recipes []any
		require.NoError(t, json.Unmarshal([]byte(call(r, "ada", http.MethodGet, "/v1/recipes", "").body), &recipes))
		require.Len(t, recipes, 4)
	})

	t.Run("the photo goes to the recipe's new ID", func(t *testing.T) {
		bread := result.RecipeIDs["r-bread"]
		res := call(r, "ada", http.MethodPost, "/v1/recipes/"+bread+"/photo/upload", `{"contentType":"image/jpeg"}`)
		require.Equal(t, http.StatusOK, res.status, res.body)
		key := res.json(t)["photoKey"].(string)
		photos.upload(key, model.StoredPhoto{Size: 1000, ContentType: "image/jpeg"})

		res = call(r, "ada", http.MethodPut, "/v1/recipes/"+bread+"/photo", fmt.Sprintf(`{"photoKey":%q}`, key))

		require.Equal(t, http.StatusOK, res.status, res.body)
		require.Equal(t, "https://cdn.test/"+key, res.json(t)["photoUrl"])
	})

	t.Run("an import from a second phone joins what's there", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/imports", importBody(uuid.NewString(), localDataV4(t)))

		require.Equal(t, http.StatusOK, res.status, res.body)
		_, categories, items, _ := catalog(t, r, "ada")
		require.Len(t, categories, len(category))
		require.Len(t, items, len(item))
		lines := call(r, "ada", http.MethodGet, "/v1/grocery-list", "").json(t)["lines"].([]any)
		require.Len(t, lines, 7)
		for _, l := range lines {
			line := l.(map[string]any)
			if line["itemId"] == item["Yeast"] {
				require.InDelta(t, 4.0, line["quantity"], 0, "amounts still to buy add up")
			}
		}
	})

	t.Run("the same import sent twice at once is imported once", func(t *testing.T) {
		id := uuid.NewString()
		var wg sync.WaitGroup
		results := make([]string, 2)
		for n := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[n] = call(r, "grace", http.MethodPost, "/v1/imports", importBody(id, localDataV4(t))).body
			}()
		}
		wg.Wait()

		var recipes []any
		require.NoError(t, json.Unmarshal([]byte(call(r, "grace", http.MethodGet, "/v1/recipes", "").body), &recipes))
		require.Len(t, recipes, 4, results)
	})

	t.Run("refuses data from another version of the app", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/imports", importBody(uuid.NewString(), `{"version":3,"recipes":[]}`))

		require.Equal(t, http.StatusBadRequest, res.status, res.body)
		require.Contains(t, res.body, "data.version")
	})
}

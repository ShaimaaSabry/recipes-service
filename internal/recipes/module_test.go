package recipes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/platform/database/databasetest"
	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// newServer runs the module on a real database. The access token is simply the owner's ID.
func newServer(t *testing.T) (*gin.Engine, *Module) {
	ctx := context.Background()
	module, err := New(databasetest.New(t))
	require.NoError(t, err)
	require.NoError(t, module.Migrate(ctx))

	r := httpserver.New(false)
	authenticate := func(_ context.Context, token string) (owner.ID, error) { return owner.ID(token), nil }
	module.RegisterRoutes(r.Group("/v1", httpserver.RequireOwner(authenticate)))
	return r, module
}

type response struct {
	status int
	header http.Header
	body   string
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.body), &decoded), r.body)
	return decoded
}

func call(r *gin.Engine, owner, method, path, body string, headers ...string) response {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+owner)
	for n := 0; n+1 < len(headers); n += 2 {
		request.Header.Set(headers[n], headers[n+1])
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	return response{status: recorder.Code, header: recorder.Header(), body: recorder.Body.String()}
}

// catalog reads the owner's catalog and indexes it: unit IDs by code, category and item IDs by name.
func catalog(t *testing.T, r *gin.Engine, owner string) (units, categories, items map[string]string, etag string) {
	t.Helper()
	res := call(r, owner, http.MethodGet, "/v1/catalog", "")
	require.Equal(t, http.StatusOK, res.status, res.body)
	var dto struct {
		Units      []struct{ ID, Code string }
		Categories []struct{ ID, Name string }
		Items      []struct{ ID, Name string }
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &dto))
	units, categories, items = map[string]string{}, map[string]string{}, map[string]string{}
	for _, u := range dto.Units {
		units[u.Code] = u.ID
	}
	for _, c := range dto.Categories {
		categories[c.Name] = c.ID
	}
	for _, i := range dto.Items {
		items[i.Name] = i.ID
	}
	return units, categories, items, res.header.Get("ETag")
}

func TestCatalog(t *testing.T) {
	r, module := newServer(t)
	ctx := context.Background()
	require.NoError(t, module.PrepareAccount(ctx, "ada"))
	require.NoError(t, module.PrepareAccount(ctx, "ada"), "preparing again changes nothing")

	unit, category, item, etag := catalog(t, r, "ada")
	require.Len(t, unit, 19)
	require.Len(t, category, 10)
	require.Len(t, item, 32)

	t.Run("the catalog answers 304 while nothing changed", func(t *testing.T) {
		res := call(r, "ada", http.MethodGet, "/v1/catalog", "", "If-None-Match", etag)

		require.Equal(t, http.StatusNotModified, res.status)
		require.Empty(t, res.body)
		require.Equal(t, etag, res.header.Get("ETag"))
	})

	t.Run("a starter item comes with its units and conversions", func(t *testing.T) {
		res := call(r, "ada", http.MethodGet, "/v1/catalog", "")
		var dto catalogDTOForTest
		require.NoError(t, json.Unmarshal([]byte(res.body), &dto))

		garlic := dto.item("Garlic")
		require.Equal(t, "system", garlic.Source)
		require.Equal(t, []string{unit["head"], unit["g"]}, garlic.BuyUnitIDs)
		require.Equal(t, []string{unit["clove"], unit["g"]}, garlic.CookUnitIDs)
		require.JSONEq(t, fmt.Sprintf(`[{"fromUnitId":%q,"toUnitId":%q,"factor":10},{"fromUnitId":%q,"toUnitId":%q,"factor":5}]`,
			unit["head"], unit["clove"], unit["clove"], unit["g"]), string(garlic.Conversions))

		cup := dto.unit("cup")
		require.Equal(t, unit["ml"], *cup.BaseUnitID)
		require.InDelta(t, 240.0, *cup.BaseSize, 0)
		require.Nil(t, dto.unit("pack").BaseUnitID)
		require.True(t, dto.unit("pack").SoldWhole)
	})

	t.Run("each account sees only its own catalog", func(t *testing.T) {
		_, theirCategories, theirItems, _ := catalog(t, r, "grace")
		require.Empty(t, theirCategories)
		require.Empty(t, theirItems)

		res := call(r, "grace", http.MethodPut, "/v1/items/"+item["Garlic"]+"/hidden", `{"hidden":true}`)
		require.Equal(t, http.StatusNotFound, res.status)
	})

	var snacks string
	t.Run("adds a category, and the catalog's ETag changes", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/categories", `{"name":" Snacks ","emoji":"🍿"}`)

		require.Equal(t, http.StatusCreated, res.status, res.body)
		created := res.json(t)
		require.Equal(t, "Snacks", created["name"])
		require.Equal(t, "user", created["source"])
		snacks = created["id"].(string)
		_, _, _, next := catalog(t, r, "ada")
		require.NotEqual(t, etag, next)
	})

	t.Run("refuses a category name that's taken, pointing at a hidden one", func(t *testing.T) {
		res := call(r, "ada", http.MethodPut, "/v1/categories/"+category["Bakery"]+"/hidden", `{"hidden":true}`)
		require.Equal(t, http.StatusOK, res.status, res.body)
		require.Equal(t, true, res.json(t)["hidden"])

		res = call(r, "ada", http.MethodPost, "/v1/categories", `{"name":"bakery","emoji":"🥖"}`)

		require.Equal(t, http.StatusConflict, res.status)
		require.Contains(t, res.body, "a hidden category called “Bakery” already exists")
	})

	var crisps string
	t.Run("adds an item with its own conversion", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Crisps","emoji":"🥔","categoryId":%q,"buyUnitIds":[%q],"cookUnitIds":[%q],
			"conversions":[{"fromUnitId":%q,"toUnitId":%q,"factor":150}]}`, snacks, unit["pack"], unit["g"], unit["pack"], unit["g"])

		res := call(r, "ada", http.MethodPost, "/v1/items", body)

		require.Equal(t, http.StatusCreated, res.status, res.body)
		crisps = res.json(t)["id"].(string)
	})

	t.Run("refuses a conversion that follows from the units", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Nuts","emoji":"🥜","categoryId":%q,"buyUnitIds":[%q],
			"conversions":[{"fromUnitId":%q,"toUnitId":%q,"factor":1000}]}`, snacks, unit["kg"], unit["kg"], unit["g"])

		res := call(r, "ada", http.MethodPost, "/v1/items", body)

		require.Equal(t, http.StatusBadRequest, res.status)
		require.Contains(t, res.body, "1 kg = ? g already follows")
	})

	t.Run("refuses an item in another account's category", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Nuts","emoji":"🥜","categoryId":%q,"buyUnitIds":[%q]}`, snacks, unit["kg"])

		res := call(r, "grace", http.MethodPost, "/v1/items", body)

		require.Equal(t, http.StatusBadRequest, res.status)
		require.Contains(t, res.body, "please choose a category")
	})

	t.Run("checks the request's shape", func(t *testing.T) {
		res := call(r, "ada", http.MethodPost, "/v1/items", `{"name":"Nuts","emoji":"🥜","categoryId":"nope","buyUnitIds":[]}`)

		require.Equal(t, http.StatusBadRequest, res.status)
		require.JSONEq(t, `{"errors":{"categoryId":"must be a UUID","buyUnitIds":"must be at least 1"}}`, res.body)
	})

	t.Run("editing without conversions keeps them", func(t *testing.T) {
		body := fmt.Sprintf(`{"name":"Crisps","emoji":"🥔","categoryId":%q,"buyUnitIds":[%q, %q],"cookUnitIds":[%q]}`,
			snacks, unit["pack"], unit["piece"], unit["g"])

		res := call(r, "ada", http.MethodPut, "/v1/items/"+crisps, body)

		require.Equal(t, http.StatusOK, res.status, res.body)
		edited := res.json(t)
		require.Len(t, edited["buyUnitIds"], 2)
		require.Len(t, edited["conversions"], 1)
	})

	t.Run("answers a missing conversion", func(t *testing.T) {
		body := fmt.Sprintf(`{"fromUnitId":%q,"toUnitId":%q,"factor":6}`, unit["pack"], unit["piece"])

		res := call(r, "ada", http.MethodPost, "/v1/items/"+crisps+"/conversions", body)

		require.Equal(t, http.StatusOK, res.status, res.body)
		require.Len(t, res.json(t)["conversions"], 2)
	})

	t.Run("a category with items in it can't be deleted", func(t *testing.T) {
		res := call(r, "ada", http.MethodDelete, "/v1/categories/"+snacks, "")

		require.Equal(t, http.StatusConflict, res.status)
		require.Contains(t, res.body, "1 grocery item uses this category")
	})

	t.Run("starter records can be hidden, not deleted", func(t *testing.T) {
		res := call(r, "ada", http.MethodDelete, "/v1/items/"+item["Garlic"], "")
		require.Equal(t, http.StatusConflict, res.status)
		require.Contains(t, res.body, "“Garlic” came with the app")

		res = call(r, "ada", http.MethodDelete, "/v1/categories/"+category["Household"], "")
		require.Equal(t, http.StatusConflict, res.status)

		res = call(r, "ada", http.MethodPut, "/v1/items/"+item["Garlic"]+"/hidden", `{"hidden":true}`)
		require.Equal(t, http.StatusOK, res.status)
		require.Equal(t, true, res.json(t)["hidden"])
	})

	t.Run("the account's own records can be deleted", func(t *testing.T) {
		require.Equal(t, http.StatusNoContent, call(r, "ada", http.MethodDelete, "/v1/items/"+crisps, "").status)
		require.Equal(t, http.StatusNoContent, call(r, "ada", http.MethodDelete, "/v1/categories/"+snacks, "").status)
		require.Equal(t, http.StatusNotFound, call(r, "ada", http.MethodDelete, "/v1/items/"+crisps, "").status)
		require.Equal(t, http.StatusNotFound, call(r, "ada", http.MethodDelete, "/v1/items/not-a-uuid", "").status)
	})

	t.Run("deleting the account's data empties its catalog", func(t *testing.T) {
		require.NoError(t, module.DeleteOwnerData(ctx, "ada"))

		_, categories, items, _ := catalog(t, r, "ada")
		require.Empty(t, categories)
		require.Empty(t, items)
	})
}

type testUnit struct {
	ID         string   `json:"id"`
	Code       string   `json:"code"`
	BaseUnitID *string  `json:"baseUnitId"`
	BaseSize   *float64 `json:"baseSize"`
	SoldWhole  bool     `json:"soldWhole"`
}

type testItem struct {
	Name        string          `json:"name"`
	Source      string          `json:"source"`
	BuyUnitIDs  []string        `json:"buyUnitIds"`
	CookUnitIDs []string        `json:"cookUnitIds"`
	Conversions json.RawMessage `json:"conversions"`
}

type catalogDTOForTest struct {
	Units []testUnit `json:"units"`
	Items []testItem `json:"items"`
}

func (c catalogDTOForTest) unit(code string) testUnit {
	for _, u := range c.Units {
		if u.Code == code {
			return u
		}
	}
	return testUnit{}
}

func (c catalogDTOForTest) item(name string) testItem {
	for _, i := range c.Items {
		if i.Name == name {
			return i
		}
	}
	return testItem{}
}

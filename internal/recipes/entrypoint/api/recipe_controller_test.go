package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipes"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type createRecipeInteractorMock struct{ mock.Mock }

func (m *createRecipeInteractorMock) Execute(_ context.Context, cmd createrecipe.Command) (*model.Recipe, error) {
	args := m.Called(cmd)
	recipe, _ := args.Get(0).(*model.Recipe)
	return recipe, args.Error(1)
}

type getRecipesInteractorMock struct{ mock.Mock }

func (m *getRecipesInteractorMock) Execute(_ context.Context, query getrecipes.Query) ([]model.Recipe, error) {
	args := m.Called(query)
	recipes, _ := args.Get(0).([]model.Recipe)
	return recipes, args.Error(1)
}

func setup() (*gin.Engine, *getRecipesInteractorMock, *createRecipeInteractorMock) {
	getRecipes, createRecipe := new(getRecipesInteractorMock), new(createRecipeInteractorMock)
	r := httpserver.New(false)
	authenticate := func(context.Context, string) (owner.ID, error) { return "device-1", nil }
	NewRecipeController(getRecipes, createRecipe).RegisterRoutes(r.Group("/v1", httpserver.RequireOwner(authenticate)))
	return r, getRecipes, createRecipe
}

func send(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer token")
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	return recorder
}

func TestCreateRecipe_201(t *testing.T) {
	r, _, createRecipe := setup()
	command := createrecipe.Command{Owner: "device-1", ID: "recipe-1", Name: "Soup"}
	createRecipe.On("Execute", command).Return(model.Of("recipe-1", "device-1", "Soup"), nil).Once()

	response := send(r, http.MethodPost, "/v1/recipes", `{"id":"recipe-1","name":"Soup"}`)

	require.Equal(t, http.StatusCreated, response.Code)
	require.JSONEq(t, `{"id":"recipe-1","name":"Soup"}`, response.Body.String())
	createRecipe.AssertExpectations(t)
}

func TestCreateRecipe_400_MissingName(t *testing.T) {
	r, _, createRecipe := setup()

	response := send(r, http.MethodPost, "/v1/recipes", `{"name":""}`)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.JSONEq(t, `{"errors":{"name":"is required"}}`, response.Body.String())
	createRecipe.AssertNotCalled(t, "Execute", mock.Anything)
}

func TestCreateRecipe_400_BlankName(t *testing.T) {
	r, _, createRecipe := setup()
	createRecipe.On("Execute", mock.Anything).Return(nil, model.ErrInvalidRecipeName).Once()

	response := send(r, http.MethodPost, "/v1/recipes", `{"name":"   "}`)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.JSONEq(t, `{"error":"invalid input: please give the recipe a name"}`, response.Body.String())
}

func TestGetRecipes_200(t *testing.T) {
	r, getRecipes, _ := setup()
	getRecipes.On("Execute", getrecipes.Query{Owner: "device-1"}).
		Return([]model.Recipe{*model.Of("recipe-1", "device-1", "Soup")}, nil).Once()

	response := send(r, http.MethodGet, "/v1/recipes", "")

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `[{"id":"recipe-1","name":"Soup"}]`, response.Body.String())
}

func TestGetRecipes_200_EmptyListNotNull(t *testing.T) {
	r, getRecipes, _ := setup()
	getRecipes.On("Execute", mock.Anything).Return([]model.Recipe{}, nil).Once()

	response := send(r, http.MethodGet, "/v1/recipes", "")

	require.JSONEq(t, `[]`, response.Body.String())
}

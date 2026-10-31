// Package recipes wires the recipes module: recipes, grocery items, units, categories and the
// grocery list. Other modules and main use only this package, never its layers directly.
package recipes

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/recipes/application/copystarterdata"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createcategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deletecategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deleteitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/editcategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/edititem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getcatalog"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipes"
	"github.com/mamienellie/recipes-service/internal/recipes/application/hidecategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/hideitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/setconversion"
	"github.com/mamienellie/recipes-service/internal/recipes/entrypoint/api"
	"github.com/mamienellie/recipes-service/internal/recipes/infrastructure/repository/sql"
	"github.com/mamienellie/recipes-service/internal/recipes/infrastructure/starterdata"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type Module struct {
	db                *gorm.DB
	recipeRepository  *sql.RecipeSqlRepository
	catalogRepository *sql.CatalogSqlRepository
	copyStarterData   *copystarterdata.Interactor
	recipeController  *api.RecipeController
	catalogController *api.CatalogController
}

func New(db *gorm.DB) (*Module, error) {
	starter, err := starterdata.Load()
	if err != nil {
		return nil, err
	}
	recipeRepository := sql.NewRecipeSqlRepository(db)
	catalog := sql.NewCatalogSqlRepository(db)
	return &Module{
		db:                db,
		recipeRepository:  recipeRepository,
		catalogRepository: catalog,
		copyStarterData:   copystarterdata.NewInteractor(catalog, starter),
		recipeController: api.NewRecipeController(
			getrecipes.NewInteractor(recipeRepository),
			createrecipe.NewInteractor(recipeRepository),
		),
		catalogController: api.NewCatalogController(api.CatalogInteractors{
			GetCatalog:     getcatalog.NewInteractor(catalog),
			CreateCategory: createcategory.NewInteractor(catalog),
			EditCategory:   editcategory.NewInteractor(catalog),
			HideCategory:   hidecategory.NewInteractor(catalog),
			DeleteCategory: deletecategory.NewInteractor(catalog),
			CreateItem:     createitem.NewInteractor(catalog),
			EditItem:       edititem.NewInteractor(catalog),
			HideItem:       hideitem.NewInteractor(catalog),
			DeleteItem:     deleteitem.NewInteractor(catalog),
			SetConversion:  setconversion.NewInteractor(catalog),
		}),
	}, nil
}

// Migrate brings the module's tables up to date.
func (m *Module) Migrate(ctx context.Context) error {
	return database.Migrate(ctx, m.db, "recipes", sql.Migrations())
}

// PrepareAccount gives the owner their copies of the starter categories and grocery items, the
// first time it runs for them. It runs on every sign-in.
func (m *Module) PrepareAccount(ctx context.Context, owner owner.ID) error {
	return m.copyStarterData.Execute(ctx, copystarterdata.Command{Owner: owner})
}

// DeleteOwnerData deletes everything the module stores for owner, when the account is deleted.
func (m *Module) DeleteOwnerData(ctx context.Context, owner owner.ID) error {
	if err := m.recipeRepository.DeleteOwnerData(ctx, owner); err != nil {
		return err
	}
	return m.catalogRepository.DeleteOwnerData(ctx, owner)
}

// RegisterRoutes adds the module's routes to r, which must sit behind httpserver.RequireOwner.
func (m *Module) RegisterRoutes(r gin.IRouter) {
	m.recipeController.RegisterRoutes(r)
	m.catalogController.RegisterRoutes(r)
}

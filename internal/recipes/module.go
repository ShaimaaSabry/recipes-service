// Package recipes wires the recipes module: recipes, grocery items, units, categories and the
// grocery list. Other modules and main use only this package, never its layers directly.
package recipes

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/recipes/application/applygroceryactions"
	"github.com/mamienellie/recipes-service/internal/recipes/application/attachphoto"
	"github.com/mamienellie/recipes-service/internal/recipes/application/copystarterdata"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createcategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deletecategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deleteitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deleterecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/editcategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/edititem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/editrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getcatalog"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getgrocerylist"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getmeals"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipes"
	"github.com/mamienellie/recipes-service/internal/recipes/application/hidecategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/hideitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/planmeal"
	"github.com/mamienellie/recipes-service/internal/recipes/application/removemeal"
	"github.com/mamienellie/recipes-service/internal/recipes/application/removephoto"
	"github.com/mamienellie/recipes-service/internal/recipes/application/requestphotoupload"
	"github.com/mamienellie/recipes-service/internal/recipes/application/setconversion"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/recipes/entrypoint/api"
	"github.com/mamienellie/recipes-service/internal/recipes/infrastructure/repository/sql"
	"github.com/mamienellie/recipes-service/internal/recipes/infrastructure/starterdata"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// PhotoStorage keeps recipe photos: photostorage.S3, or photostorage.Disabled without a bucket.
type PhotoStorage interface {
	PresignUpload(ctx context.Context, key, contentType string) (model.PhotoUpload, error)
	Stat(ctx context.Context, key string) (model.StoredPhoto, bool, error)
	Delete(ctx context.Context, keys ...string) error
	DeleteAll(ctx context.Context, prefix string) error
}

type Settings struct {
	Photos PhotoStorage
	// PhotosURL is the CloudFront address photos are served from; empty without photos.
	PhotosURL string
}

type Module struct {
	db                *gorm.DB
	photos            PhotoStorage
	recipeRepository  *sql.RecipeSqlRepository
	catalogRepository *sql.CatalogSqlRepository
	copyStarterData   *copystarterdata.Interactor
	recipeController  *api.RecipeController
	catalogController *api.CatalogController
	menuController    *api.MenuController
}

func New(db *gorm.DB, settings Settings) (*Module, error) {
	starter, err := starterdata.Load()
	if err != nil {
		return nil, err
	}
	recipes := sql.NewRecipeSqlRepository(db)
	catalog := sql.NewCatalogSqlRepository(db)
	groceryList := sql.NewGroceryListSqlRepository(db)
	photos := settings.Photos
	return &Module{
		db:                db,
		photos:            photos,
		recipeRepository:  recipes,
		catalogRepository: catalog,
		copyStarterData:   copystarterdata.NewInteractor(catalog, starter),
		recipeController: api.NewRecipeController(api.RecipeInteractors{
			GetRecipes:         getrecipes.NewInteractor(recipes),
			GetRecipe:          getrecipe.NewInteractor(recipes),
			CreateRecipe:       createrecipe.NewInteractor(catalog, recipes),
			EditRecipe:         editrecipe.NewInteractor(catalog, recipes),
			DeleteRecipe:       deleterecipe.NewInteractor(recipes, photos),
			RequestPhotoUpload: requestphotoupload.NewInteractor(recipes, photos),
			AttachPhoto:        attachphoto.NewInteractor(recipes, photos),
			RemovePhoto:        removephoto.NewInteractor(recipes, photos),
		}, settings.PhotosURL),
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
		menuController: api.NewMenuController(api.MenuInteractors{
			GetMeals:            getmeals.NewInteractor(recipes),
			PlanMeal:            planmeal.NewInteractor(recipes),
			RemoveMeal:          removemeal.NewInteractor(recipes),
			GetGroceryList:      getgrocerylist.NewInteractor(groceryList),
			ApplyGroceryActions: applygroceryactions.NewInteractor(groceryList),
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
// Photos go first: if anything fails part-way, the account still exists and deleting it again
// finishes the job.
func (m *Module) DeleteOwnerData(ctx context.Context, owner owner.ID) error {
	if err := m.photos.DeleteAll(ctx, model.OwnerPhotoPrefix(owner)); err != nil {
		return err
	}
	if err := m.recipeRepository.DeleteOwnerData(ctx, owner); err != nil {
		return err
	}
	return m.catalogRepository.DeleteOwnerData(ctx, owner)
}

// RegisterRoutes adds the module's routes to r, which must sit behind httpserver.RequireOwner.
func (m *Module) RegisterRoutes(r gin.IRouter) {
	m.recipeController.RegisterRoutes(r)
	m.catalogController.RegisterRoutes(r)
	m.menuController.RegisterRoutes(r)
}

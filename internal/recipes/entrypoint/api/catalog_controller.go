package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createcategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deletecategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deleteitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/editcategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/edititem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getcatalog"
	"github.com/mamienellie/recipes-service/internal/recipes/application/hidecategory"
	"github.com/mamienellie/recipes-service/internal/recipes/application/hideitem"
	"github.com/mamienellie/recipes-service/internal/recipes/application/setconversion"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type (
	getCatalogInteractor interface {
		Execute(ctx context.Context, query getcatalog.Query) (getcatalog.Catalog, error)
	}
	createCategoryInteractor interface {
		Execute(ctx context.Context, command createcategory.Command) (*model.Category, error)
	}
	editCategoryInteractor interface {
		Execute(ctx context.Context, command editcategory.Command) (*model.Category, error)
	}
	hideCategoryInteractor interface {
		Execute(ctx context.Context, command hidecategory.Command) (*model.Category, error)
	}
	deleteCategoryInteractor interface {
		Execute(ctx context.Context, command deletecategory.Command) error
	}
	createItemInteractor interface {
		Execute(ctx context.Context, command createitem.Command) (*model.GroceryItem, error)
	}
	editItemInteractor interface {
		Execute(ctx context.Context, command edititem.Command) (*model.GroceryItem, error)
	}
	hideItemInteractor interface {
		Execute(ctx context.Context, command hideitem.Command) (*model.GroceryItem, error)
	}
	deleteItemInteractor interface {
		Execute(ctx context.Context, command deleteitem.Command) error
	}
	setConversionInteractor interface {
		Execute(ctx context.Context, command setconversion.Command) (*model.GroceryItem, error)
	}
)

// CatalogInteractors are the use cases behind the catalog routes.
type CatalogInteractors struct {
	GetCatalog     getCatalogInteractor
	CreateCategory createCategoryInteractor
	EditCategory   editCategoryInteractor
	HideCategory   hideCategoryInteractor
	DeleteCategory deleteCategoryInteractor
	CreateItem     createItemInteractor
	EditItem       editItemInteractor
	HideItem       hideItemInteractor
	DeleteItem     deleteItemInteractor
	SetConversion  setConversionInteractor
}

type CatalogController struct {
	use CatalogInteractors
}

func NewCatalogController(interactors CatalogInteractors) *CatalogController {
	return &CatalogController{use: interactors}
}

// RegisterRoutes adds the catalog routes to r, which must sit behind httpserver.RequireOwner.
func (h *CatalogController) RegisterRoutes(r gin.IRouter) {
	r.GET("/catalog", h.GetCatalog)

	r.POST("/categories", h.CreateCategory)
	r.PUT("/categories/:id", h.EditCategory)
	r.PUT("/categories/:id/hidden", h.HideCategory)
	r.DELETE("/categories/:id", h.DeleteCategory)

	r.POST("/items", h.CreateItem)
	r.PUT("/items/:id", h.EditItem)
	r.PUT("/items/:id/hidden", h.HideItem)
	r.DELETE("/items/:id", h.DeleteItem)
	r.POST("/items/:id/conversions", h.SetConversion)
}

// GetCatalog godoc
// @Summary     Get the units, categories and grocery items
// @Description Everything the app needs to describe groceries and convert amounts offline: the
// @Description units, and the account's categories and items (hidden ones too) with their units
// @Description and conversions. Send the last ETag as If-None-Match: when nothing changed, the
// @Description answer is 304 with no body.
// @Tags        catalog
// @Security    BearerAuth
// @Param       If-None-Match header string false "ETag of the catalog the app already has"
// @Success     200 {object} catalogDTO
// @Header      200 {string} ETag "Version of this catalog"
// @Success     304 "Not modified"
// @Failure     401 {object} map[string]any
// @Router      /catalog [get]
func (h *CatalogController) GetCatalog(ctx *gin.Context) {
	catalog, err := h.use.GetCatalog.Execute(ctx.Request.Context(), getcatalog.Query{Owner: httpserver.OwnerOf(ctx)})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	body, err := json.Marshal(toCatalogDTO(catalog))
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	// The ETag is a hash of the body, so it changes exactly when anything in it does.
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`
	ctx.Header("ETag", etag)
	ctx.Header("Cache-Control", "private, no-cache")
	if matchesETag(ctx.GetHeader("If-None-Match"), etag) {
		ctx.Status(http.StatusNotModified)
		return
	}
	ctx.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// matchesETag reports whether an If-None-Match header names etag. Weak ETags (W/"…") match too,
// since proxies may weaken them.
func matchesETag(ifNoneMatch, etag string) bool {
	for candidate := range strings.SplitSeq(ifNoneMatch, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == etag || candidate == "*" {
			return true
		}
	}
	return false
}

// CreateCategory godoc
// @Summary     Add a category
// @Tags        catalog
// @Security    BearerAuth
// @Param       payload body categoryRequest true "The category"
// @Success     201 {object} categoryDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     409 {object} map[string]any "Another category has that name"
// @Router      /categories [post]
func (h *CatalogController) CreateCategory(ctx *gin.Context) {
	var request categoryRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	category, err := h.use.CreateCategory.Execute(ctx.Request.Context(), createcategory.Command{
		Owner: httpserver.OwnerOf(ctx), Name: request.Name, Emoji: request.Emoji,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusCreated, toCategoryDTO(category))
}

// EditCategory godoc
// @Summary     Rename a category
// @Description Starter categories can be renamed too.
// @Tags        catalog
// @Security    BearerAuth
// @Param       id      path string          true "Category ID"
// @Param       payload body categoryRequest true "The category"
// @Success     200 {object} categoryDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Failure     409 {object} map[string]any "Another category has that name"
// @Router      /categories/{id} [put]
func (h *CatalogController) EditCategory(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrCategoryNotFound)
	if !ok {
		return
	}
	var request categoryRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	category, err := h.use.EditCategory.Execute(ctx.Request.Context(), editcategory.Command{
		Owner: httpserver.OwnerOf(ctx), ID: id, Name: request.Name, Emoji: request.Emoji,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toCategoryDTO(category))
}

// HideCategory godoc
// @Summary     Hide or show a category
// @Description A hidden category isn't offered when picking one; its items stay in it.
// @Tags        catalog
// @Security    BearerAuth
// @Param       id      path string        true "Category ID"
// @Param       payload body hiddenRequest true "Hide or show"
// @Success     200 {object} categoryDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /categories/{id}/hidden [put]
func (h *CatalogController) HideCategory(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrCategoryNotFound)
	if !ok {
		return
	}
	var request hiddenRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	category, err := h.use.HideCategory.Execute(ctx.Request.Context(), hidecategory.Command{
		Owner: httpserver.OwnerOf(ctx), ID: id, Hidden: *request.Hidden,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toCategoryDTO(category))
}

// DeleteCategory godoc
// @Summary     Delete a category
// @Description Only the account's own categories, and only once no item is in them. Starter ones
// @Description can be hidden instead.
// @Tags        catalog
// @Security    BearerAuth
// @Param       id path string true "Category ID"
// @Success     204
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Failure     409 {object} map[string]any "A starter category, or one with items in it"
// @Router      /categories/{id} [delete]
func (h *CatalogController) DeleteCategory(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrCategoryNotFound)
	if !ok {
		return
	}
	err := h.use.DeleteCategory.Execute(ctx.Request.Context(), deletecategory.Command{Owner: httpserver.OwnerOf(ctx), ID: id})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// CreateItem godoc
// @Summary     Add a grocery item
// @Description Conversions are the item's own ("1 head = 10 clove"); standard ones (1 kg = 1000 g)
// @Description follow from the units and are refused. Each must be needed: one that already follows
// @Description from the others is refused, so two rules can't disagree.
// @Tags        catalog
// @Security    BearerAuth
// @Param       payload body createItemRequest true "The item"
// @Success     201 {object} itemDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     409 {object} map[string]any "Another item has that name"
// @Router      /items [post]
func (h *CatalogController) CreateItem(ctx *gin.Context) {
	var request createItemRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	item, err := h.use.CreateItem.Execute(ctx.Request.Context(), createitem.Command{
		Owner:       httpserver.OwnerOf(ctx),
		Details:     toItemDetails(request.Name, request.Emoji, request.CategoryID, request.BuyUnitIDs, request.CookUnitIDs),
		Conversions: toConversions(request.Conversions),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusCreated, toItemDTO(item))
}

// EditItem godoc
// @Summary     Change a grocery item
// @Description Starter items can be changed too. Leave out conversions to keep the saved ones,
// @Description minus any naming a unit the item no longer has.
// @Tags        catalog
// @Security    BearerAuth
// @Param       id      path string          true "Item ID"
// @Param       payload body editItemRequest true "The item"
// @Success     200 {object} itemDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Failure     409 {object} map[string]any "Another item has that name"
// @Router      /items/{id} [put]
func (h *CatalogController) EditItem(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrItemNotFound)
	if !ok {
		return
	}
	var request editItemRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	command := edititem.Command{
		Owner:   httpserver.OwnerOf(ctx),
		ID:      id,
		Details: toItemDetails(request.Name, request.Emoji, request.CategoryID, request.BuyUnitIDs, request.CookUnitIDs),
	}
	if request.Conversions != nil {
		conversions := toConversions(*request.Conversions)
		command.Conversions = &conversions
	}
	item, err := h.use.EditItem.Execute(ctx.Request.Context(), command)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toItemDTO(item))
}

// HideItem godoc
// @Summary     Hide or show a grocery item
// @Description A hidden item isn't offered when picking one.
// @Tags        catalog
// @Security    BearerAuth
// @Param       id      path string        true "Item ID"
// @Param       payload body hiddenRequest true "Hide or show"
// @Success     200 {object} itemDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /items/{id}/hidden [put]
func (h *CatalogController) HideItem(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrItemNotFound)
	if !ok {
		return
	}
	var request hiddenRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	item, err := h.use.HideItem.Execute(ctx.Request.Context(), hideitem.Command{
		Owner: httpserver.OwnerOf(ctx), ID: id, Hidden: *request.Hidden,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toItemDTO(item))
}

// DeleteItem godoc
// @Summary     Delete a grocery item
// @Description Only the account's own items. Starter ones can be hidden instead.
// @Tags        catalog
// @Security    BearerAuth
// @Param       id path string true "Item ID"
// @Success     204
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Failure     409 {object} map[string]any "A starter item"
// @Router      /items/{id} [delete]
func (h *CatalogController) DeleteItem(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrItemNotFound)
	if !ok {
		return
	}
	err := h.use.DeleteItem.Execute(ctx.Request.Context(), deleteitem.Command{Owner: httpserver.OwnerOf(ctx), ID: id})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// SetConversion godoc
// @Summary     Add a conversion to a grocery item
// @Description "1 fromUnit = factor toUnit" for this item, replacing the one for the same two units
// @Description (either way round) if there is one. Used to answer "1 box = ? piece".
// @Tags        catalog
// @Security    BearerAuth
// @Param       id      path string        true "Item ID"
// @Param       payload body conversionDTO true "The conversion"
// @Success     200 {object} itemDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /items/{id}/conversions [post]
func (h *CatalogController) SetConversion(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrItemNotFound)
	if !ok {
		return
	}
	var request conversionDTO
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	item, err := h.use.SetConversion.Execute(ctx.Request.Context(), setconversion.Command{
		Owner: httpserver.OwnerOf(ctx), ItemID: id, Conversion: toConversion(request),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toItemDTO(item))
}

// pathID reads the :id path parameter. One that isn't a UUID can't name a record, so it answers
// with notFound.
func pathID(ctx *gin.Context, notFound error) (uuid.UUID, bool) {
	id, err := uuid.Parse(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(notFound)
		return uuid.Nil, false
	}
	return id, true
}

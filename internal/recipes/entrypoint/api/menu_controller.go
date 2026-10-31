package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/application/applygroceryactions"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getgrocerylist"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getmeals"
	"github.com/mamienellie/recipes-service/internal/recipes/application/planmeal"
	"github.com/mamienellie/recipes-service/internal/recipes/application/removemeal"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type (
	getMealsInteractor interface {
		Execute(ctx context.Context, query getmeals.Query) ([]*model.PlannedMeal, error)
	}
	planMealInteractor interface {
		Execute(ctx context.Context, command planmeal.Command) (*model.PlannedMeal, error)
	}
	removeMealInteractor interface {
		Execute(ctx context.Context, command removemeal.Command) error
	}
	getGroceryListInteractor interface {
		Execute(ctx context.Context, query getgrocerylist.Query) (model.GroceryList, error)
	}
	applyGroceryActionsInteractor interface {
		Execute(ctx context.Context, command applygroceryactions.Command) ([]applygroceryactions.Result, model.GroceryList, error)
	}
)

// MenuInteractors are the use cases behind the upcoming menu and grocery list routes: the two
// things the app changes offline.
type MenuInteractors struct {
	GetMeals            getMealsInteractor
	PlanMeal            planMealInteractor
	RemoveMeal          removeMealInteractor
	GetGroceryList      getGroceryListInteractor
	ApplyGroceryActions applyGroceryActionsInteractor
}

type MenuController struct {
	use MenuInteractors
}

func NewMenuController(interactors MenuInteractors) *MenuController {
	return &MenuController{use: interactors}
}

// RegisterRoutes adds the menu and grocery list routes to r, which must sit behind
// httpserver.RequireOwner.
func (h *MenuController) RegisterRoutes(r gin.IRouter) {
	r.GET("/meals", h.GetMeals)
	r.PUT("/meals/:id", h.PlanMeal)
	r.DELETE("/meals/:id", h.RemoveMeal)
	r.GET("/grocery-list", h.GetGroceryList)
	r.POST("/grocery-list/actions", h.ApplyGroceryActions)
}

// GetMeals godoc
// @Summary     Get the upcoming menu
// @Description The planned meals, in the order they were planned.
// @Tags        menu
// @Security    BearerAuth
// @Success     200 {array}  mealDTO
// @Failure     401 {object} map[string]any
// @Router      /meals [get]
func (h *MenuController) GetMeals(ctx *gin.Context) {
	meals, err := h.use.GetMeals.Execute(ctx.Request.Context(), getmeals.Query{Owner: httpserver.OwnerOf(ctx)})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	dtos := make([]mealDTO, 0, len(meals))
	for _, meal := range meals {
		dtos = append(dtos, toMealDTO(meal))
	}
	ctx.JSON(http.StatusOK, dtos)
}

// PlanMeal godoc
// @Summary     Plan a meal, or change one
// @Description The app makes the meal's ID (a UUID), so it can plan meals offline and send them
// @Description later. Sending the same meal again just saves it again.
// @Tags        menu
// @Security    BearerAuth
// @Param       id      path string          true "Meal ID, made by the app"
// @Param       payload body planMealRequest true "The recipe and number of people"
// @Success     200 {object} mealDTO
// @Failure     400 {object} map[string]any "Not a UUID, or not one of the account's recipes"
// @Failure     401 {object} map[string]any
// @Failure     409 {object} map[string]any "Another account's meal has this ID"
// @Router      /meals/{id} [put]
func (h *MenuController) PlanMeal(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrInvalidMealID)
	if !ok {
		return
	}
	var request planMealRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	meal, err := h.use.PlanMeal.Execute(ctx.Request.Context(), planmeal.Command{
		Owner: httpserver.OwnerOf(ctx), ID: id, RecipeID: uuid.MustParse(request.RecipeID), Servings: request.Servings,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toMealDTO(meal))
}

// RemoveMeal godoc
// @Summary     Take a meal off the menu
// @Description The grocery list is left as it is. Removing a meal that's already gone succeeds.
// @Tags        menu
// @Security    BearerAuth
// @Param       id path string true "Meal ID"
// @Success     204
// @Failure     400 {object} map[string]any "Not a UUID"
// @Failure     401 {object} map[string]any
// @Router      /meals/{id} [delete]
func (h *MenuController) RemoveMeal(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrInvalidMealID)
	if !ok {
		return
	}
	if err := h.use.RemoveMeal.Execute(ctx.Request.Context(), removemeal.Command{Owner: httpserver.OwnerOf(ctx), ID: id}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetGroceryList godoc
// @Summary     Get the grocery list
// @Description One line per item, in the order lines were first added.
// @Tags        grocery list
// @Security    BearerAuth
// @Success     200 {object} groceryListDTO
// @Failure     401 {object} map[string]any
// @Router      /grocery-list [get]
func (h *MenuController) GetGroceryList(ctx *gin.Context) {
	list, err := h.use.GetGroceryList.Execute(ctx.Request.Context(), getgrocerylist.Query{Owner: httpserver.OwnerOf(ctx)})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, groceryListDTO{Lines: toGroceryLineDTOs(list)})
}

// ApplyGroceryActions godoc
// @Summary     Change the grocery list
// @Description Applies the actions in order, each on its own, and returns what happened to each
// @Description and the list after them. Lines are named by item:
// @Description - add {itemId, unitId, quantity}: onto the item's line, converted into its unit
// @Description - setQuantity {itemId, quantity}: the amount, by hand, in the line's unit
// @Description - setUnit {itemId, unitId}: another unit the item is bought in, converting the amount
// @Description - check / uncheck / remove {itemId}, and clearChecked
// @Description
// @Description Every action has an ID the app makes once. Resending an action already applied
// @Description changes nothing (outcome alreadyApplied), so after a dropped connection the app
// @Description just sends its queue again. A rejected action changed nothing and can be sent again
// @Description once fixed: missingConversion says which conversion to ask the user for. Up to 200
// @Description actions per request.
// @Tags        grocery list
// @Security    BearerAuth
// @Param       payload body groceryActionsRequest true "The actions, in order"
// @Success     200 {object} groceryActionsDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Router      /grocery-list/actions [post]
func (h *MenuController) ApplyGroceryActions(ctx *gin.Context) {
	var request groceryActionsRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	results, list, err := h.use.ApplyGroceryActions.Execute(ctx.Request.Context(), applygroceryactions.Command{
		Owner: httpserver.OwnerOf(ctx), Actions: toGroceryActions(request.Actions),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	dtos := make([]groceryActionResultDTO, 0, len(results))
	for _, result := range results {
		dto := groceryActionResultDTO{ID: result.ActionID.String(), Outcome: string(result.Outcome)}
		if result.Err != nil {
			message := result.Err.Error()
			dto.Error = &message
			var missing *model.MissingConversionError
			if errors.As(result.Err, &missing) {
				dto.MissingConversion = &missingConversionDTO{
					ItemID: missing.ItemID.String(), FromUnitID: missing.FromUnitID.String(), ToUnitID: missing.ToUnitID.String(),
				}
			}
		}
		dtos = append(dtos, dto)
	}
	ctx.JSON(http.StatusOK, groceryActionsDTO{Results: dtos, Lines: toGroceryLineDTOs(list)})
}

package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/application/importlocaldata"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type importLocalDataInteractor interface {
	Execute(ctx context.Context, command importlocaldata.Command) (model.ImportResult, bool, error)
}

// ImportController brings in what the app kept on the phone before accounts.
type ImportController struct {
	importLocalData importLocalDataInteractor
}

func NewImportController(importLocalData importLocalDataInteractor) *ImportController {
	return &ImportController{importLocalData: importLocalData}
}

// RegisterRoutes adds the import route to r, which must sit behind httpserver.RequireOwner.
func (h *ImportController) RegisterRoutes(r gin.IRouter) {
	r.POST("/imports", h.ImportLocalData)
}

// ImportLocalData godoc
// @Summary     Import the data the app kept on the phone
// @Description Sent once, after the first sign-in, with the app's stored data as it is (data
// @Description version 4). Starter categories and items are matched to the account's copies by
// @Description their seed IDs ("item-garlic") and take on the local changes; the user's own are
// @Description matched by name or created. Units the service doesn't have become "piece".
// @Description Recipes are created, meals not yet cooked are planned, and grocery lines join the
// @Description list. Anything that can't be kept as it is is changed or left out, and listed in
// @Description notes. It all happens or none of it does. Resending the same importId returns the
// @Description first import's result with imported false. Afterwards, upload each recipe's photo
// @Description to its new ID from recipeIds, then delete the local data.
// @Tags        imports
// @Security    BearerAuth
// @Param       payload body importRequest true "The import's ID and the app's stored data"
// @Success     200 {object} importDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Router      /imports [post]
func (h *ImportController) ImportLocalData(ctx *gin.Context) {
	var request importRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	result, imported, err := h.importLocalData.Execute(ctx.Request.Context(), importlocaldata.Command{
		Owner: httpserver.OwnerOf(ctx), ImportID: uuid.MustParse(request.ImportID), Data: toLocalData(request.Data),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toImportDTO(result, imported))
}

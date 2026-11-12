package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/attachaiB/backend-challenge/internal/model"
	"github.com/attachaiB/backend-challenge/internal/service"
)

type UserController struct {
	userService service.UserService
}

func NewUserController(userService service.UserService) *UserController {
	return &UserController{userService: userService}
}

// Create POST /api/users
func (c *UserController) Create(ctx *gin.Context) {
	var req model.CreateUserRequest
	if !bindJSON(ctx, &req) {
		return
	}

	res, err := c.userService.Create(ctx.Request.Context(), req)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, res)
}

// GetByID GET /api/users/:id
func (c *UserController) GetByID(ctx *gin.Context) {
	res, err := c.userService.GetByID(ctx.Request.Context(), ctx.Param("id"))
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// List GET /api/users
func (c *UserController) List(ctx *gin.Context) {
	res, err := c.userService.List(ctx.Request.Context())
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Update PATCH /api/users/:id
func (c *UserController) Update(ctx *gin.Context) {
	var req model.UpdateUserRequest
	if !bindJSON(ctx, &req) {
		return
	}

	res, err := c.userService.Update(ctx.Request.Context(), ctx.Param("id"), req)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

// Delete DELETE /api/users/:id
func (c *UserController) Delete(ctx *gin.Context) {
	if err := c.userService.Delete(ctx.Request.Context(), ctx.Param("id")); err != nil {
		respondError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

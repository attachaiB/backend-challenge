package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/attachaiB/backend-challenge/internal/model"
	"github.com/attachaiB/backend-challenge/internal/service"
)

type AuthController struct {
	authService service.AuthService
}

func NewAuthController(authService service.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// Register POST /api/auth/register
func (c *AuthController) Register(ctx *gin.Context) {
	var req model.CreateUserRequest
	if !bindJSON(ctx, &req) {
		return
	}

	res, err := c.authService.Register(ctx.Request.Context(), req)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, res)
}

// Login POST /api/auth/login
func (c *AuthController) Login(ctx *gin.Context) {
	var req model.LoginRequest
	if !bindJSON(ctx, &req) {
		return
	}

	res, err := c.authService.Login(ctx.Request.Context(), req)
	if err != nil {
		respondError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, res)
}

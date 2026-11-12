// Package middleware contains Gin middlewares shared by the routes.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/attachaiB/backend-challenge/internal/model"
	"github.com/attachaiB/backend-challenge/internal/service"
)

// UserIDKey is the gin.Context key that holds the logged-in user's ID.
const UserIDKey = "userID"

// Auth requires a valid "Authorization: Bearer <token>" header.
// On success it stores the user ID in the context for the controllers.
func Auth(authService service.AuthService) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		token, ok := strings.CutPrefix(ctx.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{Error: "missing bearer token"})
			return
		}

		userID, err := authService.ParseToken(token)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, model.ErrorResponse{Error: "invalid or expired token"})
			return
		}

		ctx.Set(UserIDKey, userID)
		ctx.Next()
	}
}

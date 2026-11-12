// Package controller handles HTTP: it binds requests into models, calls the
// service, and turns the result (or error) into a JSON response.
package controller

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/attachaiB/backend-challenge/internal/model"
)

// respondError maps service errors to HTTP status codes in one place,
// so every endpoint returns errors in the same shape.
func respondError(ctx *gin.Context, err error) {
	var validationErr *model.ValidationError

	switch {
	case errors.As(err, &validationErr):
		ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: validationErr.Message, Field: validationErr.Field})
	case errors.Is(err, model.ErrInvalidCredentials), errors.Is(err, model.ErrUnauthorized):
		ctx.JSON(http.StatusUnauthorized, model.ErrorResponse{Error: err.Error()})
	case errors.Is(err, model.ErrUserNotFound):
		ctx.JSON(http.StatusNotFound, model.ErrorResponse{Error: err.Error()})
	case errors.Is(err, model.ErrEmailTaken):
		ctx.JSON(http.StatusConflict, model.ErrorResponse{Error: err.Error(), Field: "email"})
	default:
		// Unknown errors are logged with details but the client only gets a generic message.
		log.Printf("internal error: %v", err)
		ctx.JSON(http.StatusInternalServerError, model.ErrorResponse{Error: "internal server error"})
	}
}

// bindJSON binds the request body and writes a 400 response if it is invalid.
// It returns false when the handler should stop.
func bindJSON(ctx *gin.Context, req any) bool {
	err := ctx.ShouldBindJSON(req)
	if err == nil {
		return true
	}

	var fieldErrors validator.ValidationErrors
	if errors.As(err, &fieldErrors) && len(fieldErrors) > 0 {
		field := fieldErrors[0]
		ctx.JSON(http.StatusBadRequest, model.ErrorResponse{
			Error: validationMessage(field),
			Field: field.Field(),
		})

		return false
	}

	ctx.JSON(http.StatusBadRequest, model.ErrorResponse{Error: "invalid JSON body"})

	return false
}

func validationMessage(field validator.FieldError) string {
	switch field.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return "must be at least " + field.Param() + " characters"
	case "max":
		return "must be at most " + field.Param() + " characters"
	default:
		return "is invalid"
	}
}

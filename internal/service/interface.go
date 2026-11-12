// Package service holds the business logic. It receives models from the
// controllers, works with entities through the repository, and returns models.
package service

import (
	"context"

	"github.com/attachaiB/backend-challenge/internal/model"
)

type AuthService interface {
	Register(ctx context.Context, req model.CreateUserRequest) (model.UserResponse, error)
	Login(ctx context.Context, req model.LoginRequest) (model.LoginResponse, error)
	// ParseToken validates a JWT and returns the user ID stored in it.
	ParseToken(token string) (string, error)
}

type UserService interface {
	Create(ctx context.Context, req model.CreateUserRequest) (model.UserResponse, error)
	GetByID(ctx context.Context, id string) (model.UserResponse, error)
	List(ctx context.Context) (model.UserListResponse, error)
	Update(ctx context.Context, id string, req model.UpdateUserRequest) (model.UserResponse, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

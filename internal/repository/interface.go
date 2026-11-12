// Package repository is the only layer that talks to MongoDB.
package repository

import (
	"context"
	"errors"

	"github.com/attachaiB/backend-challenge/internal/entity"
)

var (
	ErrNotFound       = errors.New("record not found")
	ErrDuplicateEmail = errors.New("duplicate email")
)

type UserRepository interface {
	Create(ctx context.Context, user entity.User) (entity.User, error)
	FindByID(ctx context.Context, id string) (entity.User, error)
	FindByEmail(ctx context.Context, email string) (entity.User, error)
	FindAll(ctx context.Context) ([]entity.User, error)
	Update(ctx context.Context, id string, update entity.UserUpdate) (entity.User, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}

package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/attachaiB/backend-challenge/internal/entity"
	"github.com/attachaiB/backend-challenge/internal/model"
	"github.com/attachaiB/backend-challenge/internal/repository"
)

type userService struct {
	userRepository repository.UserRepository
}

func NewUserService(userRepository repository.UserRepository) UserService {
	return &userService{userRepository: userRepository}
}

func (s *userService) Create(ctx context.Context, req model.CreateUserRequest) (model.UserResponse, error) {
	return createUser(ctx, s.userRepository, req)
}

func (s *userService) GetByID(ctx context.Context, id string) (model.UserResponse, error) {
	user, err := s.userRepository.FindByID(ctx, id)
	if err != nil {
		return model.UserResponse{}, mapRepositoryError(err)
	}

	return toUserResponse(user), nil
}

func (s *userService) List(ctx context.Context) (model.UserListResponse, error) {
	users, err := s.userRepository.FindAll(ctx)
	if err != nil {
		return model.UserListResponse{}, err
	}

	result := model.UserListResponse{
		Users: make([]model.UserResponse, 0, len(users)),
		Total: len(users),
	}
	for _, user := range users {
		result.Users = append(result.Users, toUserResponse(user))
	}

	return result, nil
}

func (s *userService) Update(
	ctx context.Context,
	id string,
	req model.UpdateUserRequest,
) (model.UserResponse, error) {
	if req.Name == nil && req.Email == nil {
		return model.UserResponse{}, &model.ValidationError{
			Field:   "body",
			Message: "name or email is required",
		}
	}

	update := entity.UserUpdate{}

	if req.Name != nil {
		name, err := normalizeName(*req.Name)
		if err != nil {
			return model.UserResponse{}, err
		}

		update.Name = &name
	}

	if req.Email != nil {
		email := normalizeEmail(*req.Email)
		update.Email = &email
	}

	user, err := s.userRepository.Update(ctx, id, update)
	if err != nil {
		return model.UserResponse{}, mapRepositoryError(err)
	}

	return toUserResponse(user), nil
}

func (s *userService) Delete(ctx context.Context, id string) error {
	return mapRepositoryError(s.userRepository.Delete(ctx, id))
}

func (s *userService) Count(ctx context.Context) (int64, error) {
	return s.userRepository.Count(ctx)
}

// createUser is shared by registration and the protected "create user" endpoint.
func createUser(
	ctx context.Context,
	userRepository repository.UserRepository,
	req model.CreateUserRequest,
) (model.UserResponse, error) {
	name, err := normalizeName(req.Name)
	if err != nil {
		return model.UserResponse{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.UserResponse{}, err
	}

	user, err := userRepository.Create(ctx, entity.User{
		Name:         name,
		Email:        normalizeEmail(req.Email),
		PasswordHash: string(hash),
		// MongoDB stores time in milliseconds, so we truncate here to return
		// the same value that a later GET will read back.
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
	})
	if err != nil {
		return model.UserResponse{}, mapRepositoryError(err)
	}

	return toUserResponse(user), nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &model.ValidationError{Field: "name", Message: "must not be blank"}
	}

	return name, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func mapRepositoryError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return model.ErrUserNotFound
	case errors.Is(err, repository.ErrDuplicateEmail):
		return model.ErrEmailTaken
	default:
		return err
	}
}

func toUserResponse(user entity.User) model.UserResponse {
	return model.UserResponse{
		ID:        user.ID.Hex(),
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

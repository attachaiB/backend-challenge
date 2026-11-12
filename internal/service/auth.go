package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/attachaiB/backend-challenge/internal/model"
	"github.com/attachaiB/backend-challenge/internal/repository"
)

// dummyHash is compared when the email does not exist, so a login for an
// unknown email takes about as long as a wrong password.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password"), bcrypt.DefaultCost)

type authService struct {
	userRepository repository.UserRepository
	secret         []byte
	duration       time.Duration
}

func NewAuthService(
	userRepository repository.UserRepository,
	secret string,
	duration time.Duration,
) AuthService {
	return &authService{
		userRepository: userRepository,
		secret:         []byte(secret),
		duration:       duration,
	}
}

func (s *authService) Register(ctx context.Context, req model.CreateUserRequest) (model.UserResponse, error) {
	return createUser(ctx, s.userRepository, req)
}

func (s *authService) Login(ctx context.Context, req model.LoginRequest) (model.LoginResponse, error) {
	user, err := s.userRepository.FindByEmail(ctx, normalizeEmail(req.Email))
	if errors.Is(err, repository.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))

		return model.LoginResponse{}, model.ErrInvalidCredentials
	}

	if err != nil {
		return model.LoginResponse{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return model.LoginResponse{}, model.ErrInvalidCredentials
	}

	now := time.Now()
	expiresAt := now.Add(s.duration)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   user.ID.Hex(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})

	signed, err := token.SignedString(s.secret)
	if err != nil {
		return model.LoginResponse{}, err
	}

	return model.LoginResponse{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt.UTC().Truncate(time.Second),
	}, nil
}

func (s *authService) ParseToken(tokenString string) (string, error) {
	claims := jwt.RegisteredClaims{}

	_, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		// Only HS256 is accepted. This blocks tokens that switch the algorithm,
		// for example to "none".
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || claims.Subject == "" {
		return "", model.ErrUnauthorized
	}

	return claims.Subject, nil
}

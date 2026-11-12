package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/attachaiB/backend-challenge/internal/entity"
	"github.com/attachaiB/backend-challenge/internal/model"
)

const testSecret = "test-secret-that-is-at-least-32-chars"

func newUserWithPassword(t *testing.T, email, password string) entity.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	user := newUser("Alice", email)
	user.PasswordHash = string(hash)

	return user
}

func signToken(t *testing.T, method jwt.SigningMethod, key any, claims jwt.Claims) string {
	t.Helper()

	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	return signed
}

func TestRegisterCreatesUser(t *testing.T) {
	repo := newFakeUserRepository()
	svc := NewAuthService(repo, testSecret, time.Hour)

	res, err := svc.Register(context.Background(), model.CreateUserRequest{
		Name: "Alice", Email: "alice@example.com", Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register error = %v", err)
	}

	if _, ok := repo.users[res.ID]; !ok {
		t.Errorf("user %s was not stored", res.ID)
	}
}

func TestLoginReturnsTokenForTheUser(t *testing.T) {
	alice := newUserWithPassword(t, "alice@example.com", "password123")
	svc := NewAuthService(newFakeUserRepository(alice), testSecret, time.Hour)

	before := time.Now()

	res, err := svc.Login(context.Background(), model.LoginRequest{
		Email: " ALICE@example.com ", Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login error = %v", err)
	}

	if res.TokenType != "Bearer" {
		t.Errorf("token type = %q, want Bearer", res.TokenType)
	}

	if res.ExpiresAt.Before(before.Add(59*time.Minute)) || res.ExpiresAt.After(before.Add(61*time.Minute)) {
		t.Errorf("expires at %v, want about one hour from now", res.ExpiresAt)
	}

	userID, err := svc.ParseToken(res.AccessToken)
	if err != nil || userID != alice.ID.Hex() {
		t.Fatalf("ParseToken = %q, %v; want %q, nil", userID, err, alice.ID.Hex())
	}
}

func TestLoginWrongPasswordAndUnknownEmailLookTheSame(t *testing.T) {
	alice := newUserWithPassword(t, "alice@example.com", "password123")
	svc := NewAuthService(newFakeUserRepository(alice), testSecret, time.Hour)

	_, wrongPassword := svc.Login(context.Background(), model.LoginRequest{
		Email: "alice@example.com", Password: "wrong-password",
	})
	_, unknownEmail := svc.Login(context.Background(), model.LoginRequest{
		Email: "nobody@example.com", Password: "password123",
	})

	if !errors.Is(wrongPassword, model.ErrInvalidCredentials) ||
		!errors.Is(unknownEmail, model.ErrInvalidCredentials) {
		t.Fatalf("got %v and %v, want ErrInvalidCredentials for both", wrongPassword, unknownEmail)
	}
}

func TestLoginPassesRepositoryError(t *testing.T) {
	dbErr := errors.New("mongo is down")
	repo := newFakeUserRepository()
	repo.err = dbErr
	svc := NewAuthService(repo, testSecret, time.Hour)

	_, err := svc.Login(context.Background(), model.LoginRequest{
		Email: "alice@example.com", Password: "password123",
	})
	if !errors.Is(err, dbErr) {
		t.Fatalf("err = %v, want the repository error, not invalid credentials", err)
	}
}

func TestParseTokenRejectsBadTokens(t *testing.T) {
	svc := NewAuthService(newFakeUserRepository(), testSecret, time.Hour)
	now := time.Now()
	valid := jwt.RegisteredClaims{
		Subject:   "6af53a00f3824b8b5e843f73",
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"not a JWT", "not.a.token"},
		{"signed with another secret", signToken(t, jwt.SigningMethodHS256, []byte("another-secret"), valid)},
		{"HS512 with the same secret", signToken(t, jwt.SigningMethodHS512, []byte(testSecret), valid)},
		{"alg none", signToken(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, valid)},
		{"expired", signToken(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.RegisteredClaims{
			Subject:   valid.Subject,
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute)),
		})},
		{"no expiry", signToken(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.RegisteredClaims{
			Subject: valid.Subject,
		})},
		{"no subject", signToken(t, jwt.SigningMethodHS256, []byte(testSecret), jwt.RegisteredClaims{
			ExpiresAt: valid.ExpiresAt,
		})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.ParseToken(tt.token); !errors.Is(err, model.ErrUnauthorized) {
				t.Errorf("err = %v, want ErrUnauthorized", err)
			}
		})
	}
}

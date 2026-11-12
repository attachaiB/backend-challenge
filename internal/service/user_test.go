package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"github.com/attachaiB/backend-challenge/internal/entity"
	"github.com/attachaiB/backend-challenge/internal/model"
)

func newUser(name, email string) entity.User {
	return entity.User{
		ID:        bson.NewObjectID(),
		Name:      name,
		Email:     email,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func strPtr(s string) *string { return &s }

func TestCreateUserHashesPasswordAndNormalizesInput(t *testing.T) {
	repo := newFakeUserRepository()
	svc := NewUserService(repo)

	res, err := svc.Create(context.Background(), model.CreateUserRequest{
		Name:     "  Alice ",
		Email:    " Alice@Example.COM ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if res.Name != "Alice" || res.Email != "alice@example.com" {
		t.Errorf("got name %q email %q, want trimmed name and lowercase email", res.Name, res.Email)
	}

	if res.ID == "" || res.CreatedAt.IsZero() {
		t.Errorf("ID and CreatedAt must be set, got %+v", res)
	}

	stored := repo.users[res.ID]
	if stored.PasswordHash == "password123" {
		t.Fatal("password was stored in plain text")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("password123")); err != nil {
		t.Errorf("stored hash does not match the password: %v", err)
	}
}

func TestCreateUserRejectsDuplicateEmail(t *testing.T) {
	repo := newFakeUserRepository(newUser("Alice", "alice@example.com"))
	svc := NewUserService(repo)

	_, err := svc.Create(context.Background(), model.CreateUserRequest{
		Name: "Other", Email: "ALICE@example.com", Password: "password123",
	})
	if !errors.Is(err, model.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestCreateUserRejectsBlankName(t *testing.T) {
	repo := newFakeUserRepository()
	svc := NewUserService(repo)

	_, err := svc.Create(context.Background(), model.CreateUserRequest{
		Name: "   ", Email: "bob@example.com", Password: "password123",
	})

	var validationErr *model.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if len(repo.users) != 0 {
		t.Errorf("stored %d users, want 0", len(repo.users))
	}
}

func TestGetUserByID(t *testing.T) {
	alice := newUser("Alice", "alice@example.com")
	svc := NewUserService(newFakeUserRepository(alice))

	res, err := svc.GetByID(context.Background(), alice.ID.Hex())
	if err != nil {
		t.Fatalf("GetByID error = %v", err)
	}

	if res.ID != alice.ID.Hex() || res.Email != alice.Email {
		t.Errorf("got %+v, want Alice", res)
	}

	if _, err := svc.GetByID(context.Background(), bson.NewObjectID().Hex()); !errors.Is(err, model.ErrUserNotFound) {
		t.Errorf("unknown ID err = %v, want ErrUserNotFound", err)
	}
}

func TestListUsers(t *testing.T) {
	svc := NewUserService(newFakeUserRepository(
		newUser("Alice", "alice@example.com"),
		newUser("Bob", "bob@example.com"),
	))

	res, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List error = %v", err)
	}

	if res.Total != 2 || len(res.Users) != 2 {
		t.Fatalf("got total %d with %d users, want 2", res.Total, len(res.Users))
	}

	if res.Users[0].Name != "Alice" || res.Users[1].Name != "Bob" {
		t.Errorf("order = %q, %q; want Alice, Bob", res.Users[0].Name, res.Users[1].Name)
	}
}

func TestListUsersEmptyIsNotNil(t *testing.T) {
	svc := NewUserService(newFakeUserRepository())

	res, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List error = %v", err)
	}

	// A nil slice would be encoded as "users": null instead of [].
	if res.Users == nil || res.Total != 0 {
		t.Errorf("got %+v, want empty non-nil list", res)
	}
}

func TestUpdateUser(t *testing.T) {
	tests := []struct {
		name      string
		req       model.UpdateUserRequest
		wantName  string
		wantEmail string
	}{
		{"name only", model.UpdateUserRequest{Name: strPtr(" Bobby ")}, "Bobby", "bob@example.com"},
		{"email only", model.UpdateUserRequest{Email: strPtr("Robert@Example.com")}, "Bob", "robert@example.com"},
		{
			"both",
			model.UpdateUserRequest{Name: strPtr("Robert"), Email: strPtr("robert@example.com")},
			"Robert",
			"robert@example.com",
		},
		{"same email as before", model.UpdateUserRequest{Email: strPtr("bob@example.com")}, "Bob", "bob@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bob := newUser("Bob", "bob@example.com")
			svc := NewUserService(newFakeUserRepository(bob))

			res, err := svc.Update(context.Background(), bob.ID.Hex(), tt.req)
			if err != nil {
				t.Fatalf("Update error = %v", err)
			}

			if res.Name != tt.wantName || res.Email != tt.wantEmail {
				t.Errorf("got %q / %q, want %q / %q", res.Name, res.Email, tt.wantName, tt.wantEmail)
			}
		})
	}
}

func TestUpdateUserSendsOnlyChangedFields(t *testing.T) {
	bob := newUser("Bob", "bob@example.com")
	repo := newFakeUserRepository(bob)
	svc := NewUserService(repo)

	if _, err := svc.Update(context.Background(), bob.ID.Hex(), model.UpdateUserRequest{Name: strPtr("Bobby")}); err != nil {
		t.Fatalf("Update error = %v", err)
	}

	if repo.lastUpdate.Email != nil {
		t.Errorf("email was sent to the repository, want only name")
	}
}

func TestUpdateUserErrors(t *testing.T) {
	alice := newUser("Alice", "alice@example.com")
	bob := newUser("Bob", "bob@example.com")

	tests := []struct {
		name  string
		id    string
		req   model.UpdateUserRequest
		check func(error) bool
	}{
		{
			"no fields",
			bob.ID.Hex(),
			model.UpdateUserRequest{},
			func(err error) bool { var v *model.ValidationError; return errors.As(err, &v) },
		},
		{
			"blank name",
			bob.ID.Hex(),
			model.UpdateUserRequest{Name: strPtr("  ")},
			func(err error) bool { var v *model.ValidationError; return errors.As(err, &v) },
		},
		{
			"email of another user",
			bob.ID.Hex(),
			model.UpdateUserRequest{Email: strPtr("ALICE@example.com")},
			func(err error) bool { return errors.Is(err, model.ErrEmailTaken) },
		},
		{
			"unknown user",
			bson.NewObjectID().Hex(),
			model.UpdateUserRequest{Name: strPtr("Ghost")},
			func(err error) bool { return errors.Is(err, model.ErrUserNotFound) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewUserService(newFakeUserRepository(alice, bob))

			_, err := svc.Update(context.Background(), tt.id, tt.req)
			if !tt.check(err) {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	bob := newUser("Bob", "bob@example.com")
	repo := newFakeUserRepository(bob)
	svc := NewUserService(repo)

	if err := svc.Delete(context.Background(), bob.ID.Hex()); err != nil {
		t.Fatalf("Delete error = %v", err)
	}

	if _, ok := repo.users[bob.ID.Hex()]; ok {
		t.Error("user still exists after delete")
	}

	if err := svc.Delete(context.Background(), bob.ID.Hex()); !errors.Is(err, model.ErrUserNotFound) {
		t.Errorf("second delete err = %v, want ErrUserNotFound", err)
	}
}

func TestCountUsers(t *testing.T) {
	svc := NewUserService(newFakeUserRepository(
		newUser("Alice", "alice@example.com"),
		newUser("Bob", "bob@example.com"),
	))

	total, err := svc.Count(context.Background())
	if err != nil || total != 2 {
		t.Fatalf("Count = %d, %v; want 2, nil", total, err)
	}
}

func TestUserServicePassesRepositoryErrors(t *testing.T) {
	dbErr := errors.New("mongo is down")
	repo := newFakeUserRepository()
	repo.err = dbErr
	svc := NewUserService(repo)
	ctx := context.Background()
	id := bson.NewObjectID().Hex()

	_, createErr := svc.Create(ctx, model.CreateUserRequest{Name: "A", Email: "a@example.com", Password: "password123"})
	_, getErr := svc.GetByID(ctx, id)
	_, listErr := svc.List(ctx)
	_, updateErr := svc.Update(ctx, id, model.UpdateUserRequest{Name: strPtr("B")})
	deleteErr := svc.Delete(ctx, id)
	_, countErr := svc.Count(ctx)

	for name, err := range map[string]error{
		"Create": createErr, "GetByID": getErr, "List": listErr,
		"Update": updateErr, "Delete": deleteErr, "Count": countErr,
	} {
		if !errors.Is(err, dbErr) {
			t.Errorf("%s err = %v, want the repository error", name, err)
		}
	}
}

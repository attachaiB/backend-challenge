package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/attachaiB/backend-challenge/internal/controller"
	"github.com/attachaiB/backend-challenge/internal/middleware"
	"github.com/attachaiB/backend-challenge/internal/model"
)

const validToken = "valid-token"

// fakeAuthService accepts only validToken and returns whatever err is set to.
type fakeAuthService struct {
	err error
}

func (f *fakeAuthService) Register(_ context.Context, req model.CreateUserRequest) (model.UserResponse, error) {
	return model.UserResponse{ID: "u1", Name: req.Name, Email: req.Email}, f.err
}

func (f *fakeAuthService) Login(context.Context, model.LoginRequest) (model.LoginResponse, error) {
	if f.err != nil {
		return model.LoginResponse{}, f.err
	}

	return model.LoginResponse{AccessToken: validToken, TokenType: "Bearer"}, nil
}

func (f *fakeAuthService) ParseToken(token string) (string, error) {
	if token != validToken {
		return "", model.ErrUnauthorized
	}

	return "u1", nil
}

// fakeUserService returns err from every method, or a fixed user when err is nil.
type fakeUserService struct {
	err        error
	gotID      string
	gotUpdate  model.UpdateUserRequest
	deleteCall int
}

func (f *fakeUserService) Create(_ context.Context, req model.CreateUserRequest) (model.UserResponse, error) {
	return model.UserResponse{ID: "u2", Name: req.Name, Email: req.Email}, f.err
}

func (f *fakeUserService) GetByID(_ context.Context, id string) (model.UserResponse, error) {
	f.gotID = id
	return model.UserResponse{ID: id, Name: "Alice"}, f.err
}

func (f *fakeUserService) List(context.Context) (model.UserListResponse, error) {
	return model.UserListResponse{Users: []model.UserResponse{{ID: "u1"}}, Total: 1}, f.err
}

func (f *fakeUserService) Update(_ context.Context, id string, req model.UpdateUserRequest) (model.UserResponse, error) {
	f.gotID, f.gotUpdate = id, req
	return model.UserResponse{ID: id}, f.err
}

func (f *fakeUserService) Delete(_ context.Context, id string) error {
	f.gotID = id
	f.deleteCall++

	return f.err
}

func (f *fakeUserService) Count(context.Context) (int64, error) { return 1, f.err }

func newTestRouter(auth *fakeAuthService, users *fakeUserService) *gin.Engine {
	gin.SetMode(gin.TestMode)

	return SetupRouter(Handlers{
		Auth:           controller.NewAuthController(auth),
		User:           controller.NewUserController(users),
		AuthMiddleware: middleware.Auth(auth),
	})
}

func do(router *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) model.ErrorResponse {
	t.Helper()

	var res model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("body %q is not an error response: %v", rec.Body.String(), err)
	}

	return res
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantField string
	}{
		{"missing name", `{"email":"a@example.com","password":"password123"}`, "name"},
		{"bad email", `{"name":"A","email":"not-an-email","password":"password123"}`, "email"},
		{"short password", `{"name":"A","email":"a@example.com","password":"short"}`, "password"},
		{"broken JSON", `{"name":`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(&fakeAuthService{}, &fakeUserService{})

			rec := do(router, http.MethodPost, "/api/auth/register", tt.body, "")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}

			if got := decodeError(t, rec).Field; got != tt.wantField {
				t.Errorf("field = %q, want %q", got, tt.wantField)
			}
		})
	}
}

func TestRegisterSuccessAndDuplicate(t *testing.T) {
	body := `{"name":"Alice","email":"alice@example.com","password":"password123"}`

	rec := do(newTestRouter(&fakeAuthService{}, &fakeUserService{}), http.MethodPost, "/api/auth/register", body, "")
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}

	auth := &fakeAuthService{err: model.ErrEmailTaken}

	rec = do(newTestRouter(auth, &fakeUserService{}), http.MethodPost, "/api/auth/register", body, "")
	if rec.Code != http.StatusConflict || decodeError(t, rec).Field != "email" {
		t.Errorf("duplicate: status = %d body = %s, want 409 on email", rec.Code, rec.Body)
	}
}

func TestLogin(t *testing.T) {
	body := `{"email":"alice@example.com","password":"password123"}`

	rec := do(newTestRouter(&fakeAuthService{}, &fakeUserService{}), http.MethodPost, "/api/auth/login", body, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"access_token"`) {
		t.Errorf("status = %d body = %s, want 200 with access_token", rec.Code, rec.Body)
	}

	auth := &fakeAuthService{err: model.ErrInvalidCredentials}

	rec = do(newTestRouter(auth, &fakeUserService{}), http.MethodPost, "/api/auth/login", body, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong password: status = %d, want 401", rec.Code)
	}
}

func TestUserEndpointsRequireToken(t *testing.T) {
	router := newTestRouter(&fakeAuthService{}, &fakeUserService{})

	endpoints := []struct{ method, path string }{
		{http.MethodPost, "/api/users"},
		{http.MethodGet, "/api/users"},
		{http.MethodGet, "/api/users/u1"},
		{http.MethodPatch, "/api/users/u1"},
		{http.MethodDelete, "/api/users/u1"},
	}

	for _, e := range endpoints {
		for _, token := range []string{"", "wrong-token"} {
			rec := do(router, e.method, e.path, `{}`, token)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s with token %q: status = %d, want 401", e.method, e.path, token, rec.Code)
			}
		}
	}
}

func TestUserEndpointsWithToken(t *testing.T) {
	users := &fakeUserService{}
	router := newTestRouter(&fakeAuthService{}, users)

	tests := []struct {
		method, path, body string
		want               int
	}{
		{http.MethodPost, "/api/users", `{"name":"Bob","email":"bob@example.com","password":"password123"}`, http.StatusCreated},
		{http.MethodGet, "/api/users", "", http.StatusOK},
		{http.MethodGet, "/api/users/u1", "", http.StatusOK},
		{http.MethodPatch, "/api/users/u1", `{"name":"Bobby"}`, http.StatusOK},
		{http.MethodDelete, "/api/users/u1", "", http.StatusNoContent},
	}

	for _, tt := range tests {
		rec := do(router, tt.method, tt.path, tt.body, validToken)
		if rec.Code != tt.want {
			t.Errorf("%s %s: status = %d, want %d (body %s)", tt.method, tt.path, rec.Code, tt.want, rec.Body)
		}
	}

	if users.gotID != "u1" || users.deleteCall != 1 {
		t.Errorf("service got id %q with %d deletes, want u1 and 1", users.gotID, users.deleteCall)
	}
}

func TestUpdatePassesOnlySentFields(t *testing.T) {
	users := &fakeUserService{}
	router := newTestRouter(&fakeAuthService{}, users)

	rec := do(router, http.MethodPatch, "/api/users/u1", `{"email":"new@example.com"}`, validToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if users.gotUpdate.Name != nil || users.gotUpdate.Email == nil || *users.gotUpdate.Email != "new@example.com" {
		t.Errorf("update = %+v, want only email", users.gotUpdate)
	}

	rec = do(router, http.MethodPatch, "/api/users/u1", `{"email":"bad"}`, validToken)
	if rec.Code != http.StatusBadRequest || decodeError(t, rec).Field != "email" {
		t.Errorf("bad email: status = %d body = %s, want 400 on email", rec.Code, rec.Body)
	}
}

func TestServiceErrorsMapToStatusCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"not found", model.ErrUserNotFound, http.StatusNotFound},
		{"email taken", model.ErrEmailTaken, http.StatusConflict},
		{"validation", &model.ValidationError{Field: "body", Message: "name or email is required"}, http.StatusBadRequest},
		{"unexpected", errors.New("connection refused to mongo:27017"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTestRouter(&fakeAuthService{}, &fakeUserService{err: tt.err})

			rec := do(router, http.MethodPatch, "/api/users/u1", `{"name":"X"}`, validToken)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestInternalErrorDoesNotLeakDetails(t *testing.T) {
	users := &fakeUserService{err: errors.New("connection refused to mongo:27017")}
	router := newTestRouter(&fakeAuthService{}, users)

	rec := do(router, http.MethodGet, "/api/users", "", validToken)
	if strings.Contains(rec.Body.String(), "mongo") {
		t.Errorf("body %s exposes the internal error", rec.Body)
	}
}

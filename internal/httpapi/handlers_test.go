package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/7-solutions/backend-challenge/internal/application"
	"github.com/7-solutions/backend-challenge/internal/auth"
	"github.com/7-solutions/backend-challenge/internal/domain"
)

func TestProtectedUsersEndpointRequiresJWT(t *testing.T) {
	server := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterLoginAndListUsers(t *testing.T) {
	server := newTestServer()

	registerBody := bytes.NewBufferString(`{"name":"Katherine Johnson","email":"katherine@example.com","password":"password123"}`)
	registerReq := httptest.NewRequest(http.MethodPost, "/register", registerBody)
	registerRec := httptest.NewRecorder()
	server.ServeHTTP(registerRec, registerReq)
	if registerRec.Code != http.StatusCreated {
		t.Fatalf("expected register 201, got %d with body %s", registerRec.Code, registerRec.Body.String())
	}

	loginBody := bytes.NewBufferString(`{"email":"katherine@example.com","password":"password123"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/login", loginBody)
	loginRec := httptest.NewRecorder()
	server.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login 200, got %d with body %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResponse struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResponse.Token == "" {
		t.Fatal("expected JWT token")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/users", nil)
	listReq.Header.Set("Authorization", "Bearer "+loginResponse.Token)
	listRec := httptest.NewRecorder()
	server.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d with body %s", listRec.Code, listRec.Body.String())
	}
}

func TestRegisterRejectsTrailingJSON(t *testing.T) {
	server := newTestServer()

	body := bytes.NewBufferString(`{"name":"Katherine Johnson","email":"katherine@example.com","password":"password123"}{}`)
	req := httptest.NewRequest(http.MethodPost, "/register", body)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRejectsOversizedJSONBody(t *testing.T) {
	server := newTestServer()

	body := strings.NewReader(`{"name":"` + strings.Repeat("a", 1<<20) + `","email":"katherine@example.com","password":"password123"}`)
	req := httptest.NewRequest(http.MethodPost, "/register", body)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", rec.Code, rec.Body.String())
	}
}

func newTestServer() http.Handler {
	repo := newHTTPMemoryRepo()
	jwtManager := auth.NewJWTManager("secret", time.Hour)
	handler := NewHandler(application.NewUserService(repo, jwtManager))
	return handler.Routes(func(next http.Handler) http.Handler { return Authenticate(jwtManager, next) })
}

type httpMemoryRepo struct {
	users map[string]domain.User
}

func newHTTPMemoryRepo() *httpMemoryRepo {
	return &httpMemoryRepo{users: map[string]domain.User{}}
}

func (r *httpMemoryRepo) Create(ctx context.Context, user *domain.User) error {
	for _, existing := range r.users {
		if existing.Email == user.Email {
			return domain.ErrEmailAlreadyExists
		}
	}
	r.users[user.ID] = *user
	return nil
}

func (r *httpMemoryRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return &user, nil
}

func (r *httpMemoryRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return &user, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *httpMemoryRepo) List(ctx context.Context) ([]domain.User, error) {
	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users, nil
}

func (r *httpMemoryRepo) Update(ctx context.Context, id string, updates domain.UpdateUserInput) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	if updates.Name != nil {
		user.Name = *updates.Name
	}
	if updates.Email != nil {
		user.Email = *updates.Email
	}
	r.users[id] = user
	return &user, nil
}

func (r *httpMemoryRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *httpMemoryRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(r.users)), nil
}

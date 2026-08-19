package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/7-solutions/backend-challenge/internal/auth"
	"github.com/7-solutions/backend-challenge/internal/domain"
)

func TestRegisterCreatesUserWithHashedPassword(t *testing.T) {
	repo := newMemoryUserRepo()
	service := NewUserService(repo, auth.NewJWTManager("pa-poy", time.Hour))

	user, err := service.Register(context.Background(), domain.CreateUserInput{
		Name:     " Bob Minion ",
		Email:    "Bob@example.com ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if user.ID == "" {
		t.Fatal("expected generated ID")
	}
	if user.Name != "Bob Minion" {
		t.Fatalf("expected trimmed name, got %q", user.Name)
	}
	if user.Email != "bob@example.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}
	if user.PasswordHash == "password123" || user.PasswordHash == "" {
		t.Fatalf("expected hashed password, got %q", user.PasswordHash)
	}
	if !auth.CheckPassword("password123", user.PasswordHash) {
		t.Fatal("hashed password should match original password")
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	repo := newMemoryUserRepo()
	service := NewUserService(repo, auth.NewJWTManager("pa-poy", time.Hour))

	input := domain.CreateUserInput{Name: "Kevin Minion", Email: "kevin@example.com", Password: "password123"}
	if _, err := service.Register(context.Background(), input); err != nil {
		t.Fatalf("first register returned error: %v", err)
	}
	if _, err := service.Register(context.Background(), input); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestLoginReturnsJWTForValidCredentials(t *testing.T) {
	repo := newMemoryUserRepo()
	jwtManager := auth.NewJWTManager("pa-poy", time.Hour)
	service := NewUserService(repo, jwtManager)

	user, err := service.Register(context.Background(), domain.CreateUserInput{
		Name:     "Kevin Minion",
		Email:    "kevin@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	result, err := service.Login(context.Background(), domain.LoginInput{
		Email:    "kevin@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}

	claims, err := jwtManager.Validate(result.Token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if claims.UserID != user.ID || claims.Email != user.Email {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestUpdateRejectsEmailUsedByAnotherUser(t *testing.T) {
	repo := newMemoryUserRepo()
	service := NewUserService(repo, auth.NewJWTManager("pa-poy", time.Hour))

	first, err := service.Register(context.Background(), domain.CreateUserInput{Name: "First", Email: "first@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("register first: %v", err)
	}
	if _, err := service.Register(context.Background(), domain.CreateUserInput{Name: "Second", Email: "second@example.com", Password: "password123"}); err != nil {
		t.Fatalf("register second: %v", err)
	}

	email := "second@example.com"
	if _, err := service.Update(context.Background(), first.ID, domain.UpdateUserInput{Email: &email}); !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

type memoryUserRepo struct {
	users map[string]domain.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{users: map[string]domain.User{}}
}

func (r *memoryUserRepo) Create(ctx context.Context, user *domain.User) error {
	for _, existing := range r.users {
		if existing.Email == user.Email {
			return domain.ErrEmailAlreadyExists
		}
	}
	r.users[user.ID] = *user
	return nil
}

func (r *memoryUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return &user, nil
}

func (r *memoryUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return &user, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (r *memoryUserRepo) List(ctx context.Context) ([]domain.User, error) {
	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	return users, nil
}

func (r *memoryUserRepo) Update(ctx context.Context, id string, updates domain.UpdateUserInput) (*domain.User, error) {
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

func (r *memoryUserRepo) Delete(ctx context.Context, id string) error {
	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *memoryUserRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(r.users)), nil
}

package application

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/7-solutions/backend-challenge/internal/auth"
	"github.com/7-solutions/backend-challenge/internal/domain"
	"github.com/7-solutions/backend-challenge/internal/ports"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserService struct {
	repo ports.UserRepository
	jwt  *auth.JWTManager
}

type AuthResult struct {
	Token string      `json:"token"`
	User  domain.User `json:"user"`
}

func NewUserService(repo ports.UserRepository, jwt *auth.JWTManager) *UserService {
	return &UserService{repo: repo, jwt: jwt}
}

func (s *UserService) Register(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	if err := validateCreateUser(input); err != nil {
		return nil, err
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))
	if _, err := s.repo.FindByEmail(ctx, normalizedEmail); err == nil {
		return nil, domain.ErrEmailAlreadyExists
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		ID:           primitive.NewObjectID().Hex(),
		Name:         strings.TrimSpace(input.Name),
		Email:        normalizedEmail,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) Login(ctx context.Context, input domain.LoginInput) (*AuthResult, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if !auth.CheckPassword(input.Password, user.PasswordHash) {
		return nil, domain.ErrInvalidCredentials
	}
	token, err := s.jwt.Generate(user.ID, user.Email)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: *user}, nil
}

func (s *UserService) Create(ctx context.Context, input domain.CreateUserInput) (*domain.User, error) {
	return s.Register(ctx, input)
}

func (s *UserService) Get(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	return s.repo.List(ctx)
}

func (s *UserService) Update(ctx context.Context, id string, input domain.UpdateUserInput) (*domain.User, error) {
	if input.Name == nil && input.Email == nil {
		return nil, validationError("at least one field is required")
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, validationError("name is required")
		}
		input.Name = &name
	}

	if input.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*input.Email))
		if !isValidEmail(email) {
			return nil, validationError("valid email is required")
		}
		if existing, err := s.repo.FindByEmail(ctx, email); err == nil && existing.ID != id {
			return nil, domain.ErrEmailAlreadyExists
		} else if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			return nil, err
		}
		input.Email = &email
	}

	return s.repo.Update(ctx, id, input)
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func validateCreateUser(input domain.CreateUserInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return validationError("name is required")
	}
	if !isValidEmail(strings.TrimSpace(input.Email)) {
		return validationError("valid email is required")
	}
	if len(input.Password) < 8 {
		return validationError("password must be at least 8 characters")
	}
	return nil
}

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", domain.ErrValidation, message)
}

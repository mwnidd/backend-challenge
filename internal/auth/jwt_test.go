package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTGenerateAndValidate(t *testing.T) {
	manager := NewJWTManager("secret", time.Hour)
	token, err := manager.Generate("user-id", "user@example.com")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if claims.UserID != "user-id" || claims.Email != "user@example.com" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestJWTRejectsUnexpectedSigningMethod(t *testing.T) {
	manager := NewJWTManager("secret", time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, Claims{UserID: "user-id"})
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("SignedString returned error: %v", err)
	}

	if _, err := manager.Validate(signed); err != ErrInvalidToken {
		t.Fatalf("expected invalid token error, got %v", err)
	}
}

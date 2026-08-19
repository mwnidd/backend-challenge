package httpapi

import (
	"context"

	"github.com/7-solutions/backend-challenge/internal/auth"
)

type contextKey string

const claimsContextKey contextKey = "claims"

func withClaims(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, claimsContextKey, claims)
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey).(*auth.Claims)
	return claims, ok
}

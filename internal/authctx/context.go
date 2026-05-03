package authctx

import "context"

type contextKey struct{}

type Claims struct {
	UserID   string
	TenantID string
	Roles    []string
}

func WithClaims(ctx context.Context, claims Claims) context.Context {
	return context.WithValue(ctx, contextKey{}, claims)
}

func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(contextKey{}).(Claims)
	return claims, ok
}

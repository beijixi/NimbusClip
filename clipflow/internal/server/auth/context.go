package auth

import "context"

// ContextKey represents a typed context key for authentication data.
type ContextKey string

const (
	// ContextKeyUser holds the authenticated user information in request context.
	ContextKeyUser ContextKey = "clipflow_user"
)

// Principal describes the authenticated request principal.
type Principal struct {
	UserID   string
	DeviceID string
}

// WithPrincipal attaches the principal to the context.
func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, ContextKeyUser, principal)
}

// PrincipalFromContext retrieves the authenticated principal from the context.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	value := ctx.Value(ContextKeyUser)
	if value == nil {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	return principal, ok
}

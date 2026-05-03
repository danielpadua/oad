package auth

import "context"

type contextKey int

const providerKey contextKey = iota

// WithProvider returns a copy of ctx carrying the resolved SCIM provider name.
// Used by handlers to know which IdP issued the request.
func WithProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, providerKey, provider)
}

// ProviderFromContext returns the SCIM provider name associated with ctx,
// if any. The boolean is false when no provider has been set or when the
// stored value is empty.
func ProviderFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(providerKey).(string)
	return v, ok && v != ""
}

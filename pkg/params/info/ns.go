package info

import "context"

var nsContextKey = contextKey("namespace")

// StoreNS stores namespace in context.
func StoreNS(ctx context.Context, ns string) context.Context {
	return context.WithValue(ctx, nsContextKey, ns)
}

// GetNS gets namespace from context.
func GetNS(ctx context.Context) string {
	if val := ctx.Value(nsContextKey); val != nil {
		if ns, ok := val.(string); ok {
			return ns
		}
	}
	return ""
}

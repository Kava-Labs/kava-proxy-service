package cachemiddleware

import "context"

// IsRequestCached returns true if the request was served from cache, for use in
// any middleware that runs after the cache middleware.
func IsRequestCached(ctx context.Context) bool {
	cached, ok := ctx.Value(CachedContextKey).(bool)
	return ok && cached
}

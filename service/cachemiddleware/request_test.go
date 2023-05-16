package cachemiddleware_test

import (
	"context"
	"testing"

	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestIsRequestCached(t *testing.T) {
	ctx := context.Background()
	require.False(t, cachemiddleware.IsRequestCached(ctx))

	cachedContext := context.WithValue(ctx, cachemiddleware.CachedContextKey, true)
	require.True(t, cachemiddleware.IsRequestCached(cachedContext))
}

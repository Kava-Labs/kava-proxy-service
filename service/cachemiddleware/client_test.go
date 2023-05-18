package cachemiddleware_test

import (
	"context"
	"testing"
	"time"

	"github.com/kava-labs/kava-proxy-service/clients/cache"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestCacheClient(t *testing.T) {
	client := cachemiddleware.NewClient(
		cache.NewInMemoryCache(),
		nil, // eth client
		time.Minute,
		"key",
		nil, // logger
	)

	data, found, shouldCache := client.GetCachedRequest(
		context.Background(),
		"host",
		&decode.EVMRPCRequestEnvelope{
			JSONRPCVersion: "2.0",
			ID:             1,
			Method:         "eth_getBlockByHash",
		},
	)

	require.Nil(t, data)
	require.False(t, found)
	require.False(t, shouldCache)
}

package cachemiddleware_test

import (
	"testing"

	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestIsBodyCacheable_Valid(t *testing.T) {
	body := testResponses[TestResponse_EthBlockByNumber_Specific].ResponseBody

	isCacheable, err := cachemiddleware.IsBodyCacheable([]byte(body))

	require.NoError(t, err)
	require.True(t, isCacheable)
}

func TestUnitTestIsBodyCacheable_NullResponse(t *testing.T) {
	// Result: null
	body := testResponses[TestResponse_EthBlockByNumber_Future].ResponseBody
	isCacheable, err := cachemiddleware.IsBodyCacheable([]byte(body))

	require.Error(t, err)
	require.Equal(t, "response is empty", err.Error())
	require.False(t, isCacheable)
}

func TestUnitTestIsBodyCacheable_ErrorResponse(t *testing.T) {
	// Result: null
	body := testResponses[TestResponse_EthBlockByNumber_Error].ResponseBody
	isCacheable, err := cachemiddleware.IsBodyCacheable([]byte(body))

	require.Error(t, err)
	require.Equal(t, "response has error: parse error (code: -32700)", err.Error())
	require.False(t, isCacheable)
}

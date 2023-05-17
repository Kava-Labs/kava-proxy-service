package cachemiddleware_test

import (
	"testing"

	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestIsBodyCacheable_Valid(t *testing.T) {
	body := testResponses[TestResponse_EthBlockByNumber_Specific].ResponseBody

	err := cachemiddleware.CheckBodyCacheable([]byte(body))

	require.NoError(t, err)
}

func TestUnitTestIsBodyCacheable_NullResponse(t *testing.T) {
	// Result: null
	body := testResponses[TestResponse_EthBlockByNumber_Future].ResponseBody
	err := cachemiddleware.CheckBodyCacheable([]byte(body))

	require.Error(t, err)
	require.Equal(t, "response is empty", err.Error())
}

func TestUnitTestIsBodyCacheable_ErrorResponse(t *testing.T) {
	// Result: null
	body := testResponses[TestResponse_EthBlockByNumber_Error].ResponseBody
	err := cachemiddleware.CheckBodyCacheable([]byte(body))

	require.Error(t, err)
	require.Equal(t, "response has error: parse error (code: -32700)", err.Error())
}

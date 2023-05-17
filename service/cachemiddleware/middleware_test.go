package cachemiddleware_test

import (
	"testing"

	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestIsBodyCacheable_Valid(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "found blockByNumber",
			body: testResponses[TestResponse_EthBlockByNumber_Specific].ResponseBody,
		},
		{
			name: "positive getBalance",
			body: testResponses[TestResponse_EthGetBalance_Positive].ResponseBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := cachemiddleware.CheckBodyCacheable([]byte(tc.body))
			require.NoError(t, err)
		})
	}
}

func TestUnitTestIsBodyCacheable_EmptyResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "null",
			body: testResponses[TestResponse_EthBlockByNumber_Future].ResponseBody,
		},
		{
			name: "0x0",
			body: testResponses[TestResponse_EthGetBalance_Zero].ResponseBody,
		},
		{
			name: "0x",
			body: testResponses[TestResponse_EthGetCode_Empty].ResponseBody,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := cachemiddleware.CheckBodyCacheable([]byte(tc.body))
			require.Error(t, err)
			require.Equal(t, "response is empty", err.Error())
		})
	}
}

func TestUnitTestIsBodyCacheable_ErrorResponse(t *testing.T) {
	// Result: null
	body := testResponses[TestResponse_EthBlockByNumber_Error].ResponseBody
	err := cachemiddleware.CheckBodyCacheable([]byte(body))

	require.Error(t, err)
	require.Equal(t, "response has error: parse error (code: -32700)", err.Error())
}

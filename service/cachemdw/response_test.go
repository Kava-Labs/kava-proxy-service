package cachemdw_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

func TestUnitTestJsonRpcResponse_IsResultEmpty(t *testing.T) {
	toJSON := func(t *testing.T, result any) []byte {
		resultInJSON, err := json.Marshal(result)
		require.NoError(t, err)

		return resultInJSON
	}

	mkResp := func(result []byte) *cachemdw.JsonRpcResponse {
		return &cachemdw.JsonRpcResponse{
			Version: "2.0",
			ID:      []byte("1"),
			Result:  result,
		}
	}

	tests := []struct {
		name    string
		resp    *cachemdw.JsonRpcResponse
		isEmpty bool
	}{
		{
			name:    "empty result",
			resp:    mkResp([]byte("")),
			isEmpty: true,
		},
		{
			name:    "invalid json",
			resp:    mkResp([]byte("invalid json")),
			isEmpty: true,
		},
		{
			name:    "empty slice",
			resp:    mkResp(toJSON(t, []interface{}{})),
			isEmpty: true,
		},
		{
			name:    "empty string",
			resp:    mkResp(toJSON(t, "")),
			isEmpty: true,
		},
		{
			name:    "0x0 string",
			resp:    mkResp(toJSON(t, "0x0")),
			isEmpty: true,
		},
		{
			name:    "0x string",
			resp:    mkResp(toJSON(t, "0x")),
			isEmpty: true,
		},
		{
			name:    "empty bool",
			resp:    mkResp(toJSON(t, false)),
			isEmpty: true,
		},
		{
			name:    "nil",
			resp:    mkResp(nil),
			isEmpty: true,
		},
		{
			name:    "null",
			resp:    mkResp(toJSON(t, nil)),
			isEmpty: true,
		},
		{
			name:    "non-empty slice",
			resp:    mkResp(toJSON(t, []interface{}{1})),
			isEmpty: false,
		},
		{
			name:    "non-empty string",
			resp:    mkResp(toJSON(t, "0x1234")),
			isEmpty: false,
		},
		{
			name:    "non-empty bool",
			resp:    mkResp(toJSON(t, true)),
			isEmpty: false,
		},
		{
			name:    "unsupported empty object",
			resp:    mkResp(toJSON(t, map[string]interface{}{})),
			isEmpty: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(
				t,
				tc.isEmpty,
				tc.resp.IsResultEmpty(),
			)
		})
	}
}

func TestUnitTestJsonRpcResponse_IsCacheable(t *testing.T) {
	toJSON := func(t *testing.T, result any) []byte {
		resultInJSON, err := json.Marshal(result)
		require.NoError(t, err)

		return resultInJSON
	}

	tests := []struct {
		name        string
		resp        *cachemdw.JsonRpcResponse
		isCacheable bool
	}{
		{
			name: "empty result",
			resp: &cachemdw.JsonRpcResponse{
				Version: "2.0",
				ID:      []byte("1"),
				Result:  []byte{},
			},
			isCacheable: false,
		},
		{
			name: "non-empty error",
			resp: &cachemdw.JsonRpcResponse{
				Version: "2.0",
				ID:      []byte("1"),
				Result:  toJSON(t, "0x1234"),
				JsonRpcError: &cachemdw.JsonRpcError{
					Code:    1,
					Message: "error",
				},
			},
			isCacheable: false,
		},
		{
			name: "valid response",
			resp: &cachemdw.JsonRpcResponse{
				Version: "2.0",
				ID:      []byte("1"),
				Result:  toJSON(t, "0x1234"),
			},
			isCacheable: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(
				t,
				tc.isCacheable,
				tc.resp.IsCacheable(),
			)
		})
	}
}

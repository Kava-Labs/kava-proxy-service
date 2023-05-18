package cachemiddleware_test

import (
	"encoding/json"
	"testing"

	"github.com/kava-labs/kava-proxy-service/service/cachemiddleware"
	"github.com/stretchr/testify/require"
)

func TestUnitTestJsonRpcMessage(t *testing.T) {
	tests := []struct {
		name               string
		giveJsonString     string
		wantJsonRpcMessage cachemiddleware.JsonRpcMessage
		wantShouldErr      bool
		wantErr            error
	}{
		{
			name: "basic",
			// Whitespace matters for fields that use json.RawMessage like params
			giveJsonString: `{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "eth_getBlockByHash",
				"params": ["0x1234",true]
			}`,
			wantJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Version: "2.0",
				ID:      toJsonRawMessage(t, 1),
				Method:  "eth_getBlockByHash",
				Params:  toJsonRawMessage(t, []interface{}{"0x1234", true}),
			},
			wantShouldErr: false,
		},
		{
			name: "basic with result",
			giveJsonString: `{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "eth_getBlockByHash",
				"params": ["0x1234",true],
				"result": "0x1234"
			}`,
			wantJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Version: "2.0",
				ID:      toJsonRawMessage(t, 1),
				Method:  "eth_getBlockByHash",
				Params:  toJsonRawMessage(t, []interface{}{"0x1234", true}),
				Result:  toJsonRawMessage(t, "0x1234"),
			},
			wantShouldErr: false,
		},
	}

	for _, tc := range tests {
		msg, err := cachemiddleware.UnmarshalJsonRpcMessage([]byte(tc.giveJsonString))

		t.Run(tc.name, func(t *testing.T) {
			if tc.wantShouldErr {
				require.Error(t, err)
				require.Equal(t, tc.wantErr, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantJsonRpcMessage, msg)
			}
		})

		t.Run(tc.name+"-marshal", func(t *testing.T) {
			if tc.wantShouldErr {
				return
			}

			bz, err := msg.Marshal()

			require.NoError(t, err)
			require.JSONEq(t, tc.giveJsonString, string(bz))
		})
	}
}

func TestUnitTestJsonRpcMessage_IsEmpty(t *testing.T) {
	tests := []struct {
		name               string
		giveJsonRpcMessage cachemiddleware.JsonRpcMessage
		wantIsEmpty        bool
	}{
		{
			name: "non-empty hex string",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: toJsonRawMessage(t, "0x1234"),
			},
			wantIsEmpty: false,
		},
		{
			name: "non-empty bool",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: toJsonRawMessage(t, true),
			},
			wantIsEmpty: false,
		},
		{
			name: "nil",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: nil,
			},
			wantIsEmpty: true,
		},
		{
			name: "invalid json",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: []byte("invalid json"),
			},
			wantIsEmpty: true,
		},
		{
			name: "empty string",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: toJsonRawMessage(t, ""),
			},
			wantIsEmpty: true,
		},
		{
			name: "empty array",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: toJsonRawMessage(t, []interface{}{}),
			},
			wantIsEmpty: true,
		},
		{
			name: "empty null",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: toJsonRawMessage(t, nil),
			},
			wantIsEmpty: true,
		},
		{
			name: "unsupported empty object",
			giveJsonRpcMessage: cachemiddleware.JsonRpcMessage{
				Result: toJsonRawMessage(t, map[string]interface{}{}),
			},
			wantIsEmpty: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Ignore error, just for output if fails
			bz, _ := json.Marshal(tc.giveJsonRpcMessage.Result)

			require.Equalf(
				t,
				tc.wantIsEmpty,
				tc.giveJsonRpcMessage.IsResultEmpty(),
				"expected IsResultEmpty to return %v with result of '%v' (bytes: %v)",
				tc.wantIsEmpty,
				bz,
				tc.giveJsonRpcMessage.Result,
			)
		})
	}
}

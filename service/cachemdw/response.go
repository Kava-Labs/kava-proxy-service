package cachemdw

import (
	"encoding/json"
	"errors"
	"fmt"
)

type JsonRpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// String returns the string representation of the error
func (e *JsonRpcError) String() string {
	return fmt.Sprintf("%s (code: %d)", e.Message, e.Code)
}

// JsonRpcResponse is a EVM JSON-RPC response
type JsonRpcResponse struct {
	Version      string          `json:"jsonrpc,omitempty"`
	ID           json.RawMessage `json:"id,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	JsonRpcError *JsonRpcError   `json:"error,omitempty"`
}

// UnmarshalJsonRpcResponse unmarshals a JSON-RPC response
func UnmarshalJsonRpcResponse(data []byte) (*JsonRpcResponse, error) {
	var msg JsonRpcResponse
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// Marshal marshals a JSON-RPC response to JSON
func (resp *JsonRpcResponse) Marshal() ([]byte, error) {
	return json.Marshal(resp)
}

// Error returns the json-rpc error if any
func (resp *JsonRpcResponse) Error() error {
	if resp.JsonRpcError == nil {
		return nil
	}

	return errors.New(resp.JsonRpcError.String())
}

// IsResultEmpty checks if the response's result is empty
func (resp *JsonRpcResponse) IsResultEmpty() bool {
	if len(resp.Result) == 0 {
		// empty response's result
		return true
	}

	var result interface{}
	err := json.Unmarshal(resp.Result, &result)
	if err != nil {
		// consider result as empty if it's malformed
		return true
	}

	switch r := result.(type) {
	case []interface{}:
		// consider result as empty if it's empty slice
		return len(r) == 0
	case string:
		// Matches:
		// - "" - Empty string
		// - "0x0" - Represents zero in official json-rpc conventions. See:
		// https://ethereum.org/en/developers/docs/apis/json-rpc/#conventions
		//
		// - "0x" - Empty response from some endpoints like getCode

		return r == "" || r == "0x0" || r == "0x"
	case bool:
		// consider result as empty if it's false
		return !r
	case nil:
		// consider result as empty if it's null
		return true
	default:
		return false
	}
}

// IsCacheable returns true in case of:
// - json-rpc response doesn't contain an error
// - json-rpc response's result isn't empty
func (resp *JsonRpcResponse) IsCacheable() bool {
	if err := resp.Error(); err != nil {
		return false
	}

	if resp.IsResultEmpty() {
		return false
	}

	return true
}

func (resp *JsonRpcResponse) IsFinal(method string) bool {
	switch method {
	case "eth_getTransactionByHash":
		var tx tx
		if err := json.Unmarshal(resp.Result, &tx); err != nil {
			return false
		}

		return tx.IsIncludedInBlock()
	default:
		return true
	}
}

type tx struct {
	BlockHash        interface{} `json:"blockHash"`
	BlockNumber      interface{} `json:"blockNumber"`
	From             string      `json:"from"`
	Gas              string      `json:"gas"`
	GasPrice         string      `json:"gasPrice"`
	Hash             string      `json:"hash"`
	Input            string      `json:"input"`
	Nonce            string      `json:"nonce"`
	To               string      `json:"to"`
	TransactionIndex interface{} `json:"transactionIndex"`
	Value            string      `json:"value"`
	Type             string      `json:"type"`
	ChainId          string      `json:"chainId"`
	V                string      `json:"v"`
	R                string      `json:"r"`
	S                string      `json:"s"`
}

func (tx *tx) IsIncludedInBlock() bool {
	return tx.BlockHash != nil &&
		tx.BlockHash != "" &&
		tx.BlockNumber != nil &&
		tx.BlockNumber != 0 &&
		tx.TransactionIndex != nil
}

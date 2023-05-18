package cachemiddleware

import (
	"encoding/json"
	"errors"
	"fmt"
)

// JsonError is a JSON-RPC error
type JsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// String returns the string representation of the error
func (e *JsonError) String() string {
	return fmt.Sprintf("%s (code: %d)", e.Message, e.Code)
}

// JsonRpcMessage is a JSON-RPC response message
type JsonRpcMessage struct {
	Version   string          `json:"jsonrpc,omitempty"`
	ID        json.RawMessage `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	JsonError *JsonError      `json:"error,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

// UnmarshalJsonRpcMessage unmarshals a JSON-RPC message
func UnmarshalJsonRpcMessage(data []byte) (JsonRpcMessage, error) {
	var msg JsonRpcMessage
	err := json.Unmarshal(data, &msg)
	return msg, err
}

// Marshal marshals the message to JSON
func (msg *JsonRpcMessage) Marshal() ([]byte, error) {
	return json.Marshal(msg)
}

// IsResultEmpty returns true if the result is considered empty. This may mean
// either the requested data was not found or if the data is actually 0.
func (msg *JsonRpcMessage) IsResultEmpty() bool {
	if len(msg.Result) == 0 {
		return true
	}

	var result interface{}
	err := json.Unmarshal(msg.Result, &result)
	if err != nil {
		// If we can't unmarshal the result, assume it's empty to prevent caching.
		return true
	}

	switch r := result.(type) {
	case []interface{}:
		// []
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
		// false
		return !r
	case nil:
		// null represented as nil in the json.RawMessage type
		return true
	default:
		return false
	}
}

// Error returns the error message if there is one
func (msg *JsonRpcMessage) Error() error {
	if msg.JsonError == nil {
		return nil
	}

	return errors.New(msg.JsonError.String())
}

// CheckCacheable returns nil if the response should be cached
func (msg *JsonRpcMessage) CheckCacheable() error {
	// Only cache if the response has no error and a non-empty result
	// Check if there was an error in response
	if err := msg.Error(); err != nil {
		return fmt.Errorf("message contains error: %w", err)
	}

	// Check if the response is empty. This also includes blocks in the future,
	// assuming the response for future blocks is empty.
	if msg.IsResultEmpty() {
		return errors.New("empty result")
	}

	return nil
}

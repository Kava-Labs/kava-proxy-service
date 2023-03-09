package decode

import "encoding/json"

// EVMRPCRequest wraps expected values present in a request
// to the RPC endpoint for an EVM node API
type EVMRPCRequestEnvelope struct {
	// version of the RPC spec being used
	// https://www.jsonrpc.org/specification
	JSONRPCVersion string `json:"jsonrpc"`
	ID             int64
	Method         string
	Params         []interface{} // a.k.a. raw json that should be parsed to a type based off the methond
}

func DecodeEVMRPCRequest(body []byte) (*EVMRPCRequestEnvelope, error) {
	var request EVMRPCRequestEnvelope
	err := json.Unmarshal(body, &request)
	return &request, err
}

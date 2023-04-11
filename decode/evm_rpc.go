package decode

import (
	"encoding/json"
	"errors"
	"fmt"

	cosmosmath "cosmossdk.io/math"
)

// Errors that might result from decoding parts or the whole of
// an EVM RPC request
var (
	ErrInvalidEthAPIRequest  = errors.New("request is not valid for the eth api")
	ErrUncachaebleEthRequest = fmt.Errorf("request is not cache-able, current cache-able requests are %s", CacheableEthMethods)
)

// (IN-PROGRESS) list of evm methods that can potentially be cached
// and so are useful for tracking the block number associated with
// any requests invoking those methods
var CacheableEthMethods = []string{
	"eth_getBlockByNumber",
}

// Mapping of string tag values used in the eth api to
// normalized int values that can be stored as the block number
// for the proxied request metric
var BlockTagToNumberCodec = map[string]int64{
	"latest":   -1,
	"pending":  -2,
	"earliest": -3,
}

// EVMRPCRequest wraps expected values present in a request
// to the RPC endpoint for an EVM node API
// https://ethereum.org/en/developers/docs/apis/json-rpc/
type EVMRPCRequestEnvelope struct {
	// version of the RPC spec being used
	// https://www.jsonrpc.org/specification
	JSONRPCVersion string `json:"jsonrpc"`
	ID             int64
	Method         string
	Params         []interface{}
}

// DecodeEVMRPCRequest attempts to decode the provided bytes into
// an EVMRPCRequestEnvelope for use by the service to extract request details
// and create an enriched request metric, returning the decoded request and error (if any)
func DecodeEVMRPCRequest(body []byte) (*EVMRPCRequestEnvelope, error) {
	var request EVMRPCRequestEnvelope
	err := json.Unmarshal(body, &request)
	return &request, err
}

// ExtractBlockNumberFromEVMRPCRequest attempts to extract the block number
// associated with a request if
// - the request is a valid evm rpc request
// - the method for the request supports specifying a block number
// - the provided block number is a valid tag or number
func (r *EVMRPCRequestEnvelope) ExtractBlockNumberFromEVMRPCRequest() (int64, error) {
	// only attempt to extract block number from a valid etherum api request
	if r.Method == "" {
		return 0, ErrInvalidEthAPIRequest
	}

	// parse block number using heuristics so byzantine
	// they require their own consensus engine 😅
	// https://ethereum.org/en/developers/docs/apis/json-rpc
	// or at least a healthy level of [code coverage](./evm_rpc_test.go) ;-)
	var blockNumber int64
	requestParams := r.Params

	switch r.Method {
	case "eth_getBlockByNumber":
		// https://ethereum.org/en/developers/docs/apis/json-rpc/#eth_getblockbynumber
		tag, isString := requestParams[0].(string)

		if !isString {
			return 0, fmt.Errorf(fmt.Sprintf("error decoding block number param from params %+v", requestParams))
		}

		tagEncoding, exists := BlockTagToNumberCodec[tag]

		if !exists {
			spaceint, valid := cosmosmath.NewIntFromString(tag)

			if !valid {
				return 0, fmt.Errorf(fmt.Sprintf("unable to parse tag %s to intege", tag))
			}

			blockNumber = spaceint.Int64()

			break
		}

		blockNumber = tagEncoding

	default:
		return 0, ErrUncachaebleEthRequest
	}

	return blockNumber, nil
}

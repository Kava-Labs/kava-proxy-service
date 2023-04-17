package decode

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/ethclient"

	cosmosmath "cosmossdk.io/math"
)

// Errors that might result from decoding parts or the whole of
// an EVM RPC request
var (
	ErrInvalidEthAPIRequest               = errors.New("request is not valid for the eth api")
	ErrUncachaebleEthRequest              = fmt.Errorf("request is not cache-able, current cache-able requests are %s", CacheableEthMethods)
	ErrUncachaebleByBlockNumberEthRequest = fmt.Errorf("request is not cache-able by block number, current cache-able requests by block number are %s", CacheableByBlockNumberMethods)
)

// List of evm methods that can be cached by block number
// and so are useful for tracking the block number associated with
// any requests invoking those methods
var CacheableByBlockNumberMethods = []string{
	"eth_getBalance",
	"eth_getStorageAt",
	"eth_getTransactionCount",
	"eth_getBlockTransactionCountByNumber",
	"eth_getUncleCountByBlockNumber",
	"eth_getCode",
	"eth_getBlockByNumber",
	"eth_getTransactionByBlockNumberAndIndex",
	"eth_getUncleByBlockNumberAndIndex",
}

// List of evm methods that can be cached by block hash
// and so are useful for converting and tracking the block hash associated with
// any requests invoking those methods to the matching block number
var CacheableByBlockHashMethods = []string{
	"eth_getBlockTransactionCountByHash",
	"eth_getUncleCountByBlockHash",
	"eth_getBlockByHash",
	"eth_getUncleByBlockHashAndIndex",
	"eth_getTransactionByBlockHashAndIndex",
}

// List of evm methods that can be cached independent
// of block number (i.e. by block or transaction hash, filter id, or time period)
// TODO: break these out into separate list for methods that can be cached using the same key type
var OtherCacheableMethods = []string{
	"web3_clientVersion",
	"web3_sha3",
	"net_version",
	"net_listening",
	"net_peerCount",
	"eth_protocolVersion",
	"eth_syncing",
	"eth_coinbase",
	"eth_mining",
	"eth_hashrate",
	"eth_gasPrice",
	"eth_accounts",
	"eth_blockNumber",
	"eth_getTransactionByHash",
	"eth_getTransactionReceipt",
	"eth_getCompilers",
	"eth_getFilterChanges",
	"eth_getFilterLogs",
	"eth_getLogs",
	"eth_getWork",
}

// List of evm methods that can be cached
// and so are useful for tracking the params
// associated with the request to help in making
// caching decisions for future similar requests
var CacheableEthMethods = append(append(CacheableByBlockNumberMethods, CacheableByBlockHashMethods...), OtherCacheableMethods...)

// Mapping of the position of the block number param for a given method name
var MethodNameToBlockNumberParamIndex = map[string]int{
	"eth_getBalance":                          1,
	"eth_getStorageAt":                        2,
	"eth_getTransactionCount":                 1,
	"eth_getBlockTransactionCountByNumber":    0,
	"eth_getUncleCountByBlockNumber":          0,
	"eth_getCode":                             1,
	"eth_getBlockByNumber":                    0,
	"eth_getTransactionByBlockNumberAndIndex": 0,
	"eth_getUncleByBlockNumberAndIndex":       1,
}

// Mapping of the position of the block hash param for a given method name
var MethodNameToBlockHashParamIndex = map[string]int{
	"eth_getBlockTransactionCountByHash":    0,
	"eth_getUncleCountByBlockHash":          0,
	"eth_getBlockByHash":                    0,
	"eth_getUncleByBlockHashAndIndex":       0,
	"eth_getTransactionByBlockHashAndIndex": 0,
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
func (r *EVMRPCRequestEnvelope) ExtractBlockNumberFromEVMRPCRequest(evmClient *ethclient.Client) (int64, error) {
	// only attempt to extract block number from a valid ethereum api request
	if r.Method == "" {
		return 0, ErrInvalidEthAPIRequest
	}

	// validate this is a request with a block number param
	var cacheableByBlockNumber bool
	for _, cacheableByBlockNumberMethod := range CacheableByBlockNumberMethods {
		if r.Method == cacheableByBlockNumberMethod {
			cacheableByBlockNumber = true
			break
		}
	}

	if !cacheableByBlockNumber {
		return 0, ErrUncachaebleByBlockNumberEthRequest
	}

	// parse block number using heuristics so byzantine
	// they require their own consensus engine 😅
	// https://ethereum.org/en/developers/docs/apis/json-rpc
	// or at least a healthy level of [code coverage](./evm_rpc_test.go) ;-)
	return parseBlockNumberFromParams(r.Method, r.Params)

}

// Generic method to parse the block number from a set of params
func parseBlockNumberFromParams(methodName string, params []interface{}) (int64, error) {
	paramIndex, exists := MethodNameToBlockNumberParamIndex[methodName]

	if !exists {
		return 0, ErrUncachaebleByBlockNumberEthRequest
	}

	tag, isString := params[paramIndex].(string)

	if !isString {
		return 0, fmt.Errorf(fmt.Sprintf("error decoding block number param from params %+v at index %d", params, paramIndex))
	}

	var blockNumber int64
	tagEncoding, exists := BlockTagToNumberCodec[tag]

	if !exists {
		spaceint, valid := cosmosmath.NewIntFromString(tag)

		if !valid {
			return 0, fmt.Errorf(fmt.Sprintf("unable to parse tag %s to integer", tag))
		}

		blockNumber = spaceint.Int64()

		return blockNumber, nil
	}

	blockNumber = tagEncoding

	return blockNumber, nil
}

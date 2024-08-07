package decode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	cosmosmath "cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/common"
	ethctypes "github.com/ethereum/go-ethereum/core/types"
)

// EVMBlockGetter defines an interface which can be implemented by any client capable of getting ethereum block header by hash
type EVMBlockGetter interface {
	// HeaderByHash returns ethereum block header by hash
	HeaderByHash(ctx context.Context, hash common.Hash) (*ethctypes.Header, error)
}

// These block tags are special strings used to reference blocks in JSON-RPC
// see https://ethereum.org/en/developers/docs/apis/json-rpc/#default-block
const (
	BlockTagLatest    = "latest"
	BlockTagPending   = "pending"
	BlockTagEarliest  = "earliest"
	BlockTagFinalized = "finalized"
	BlockTagSafe      = "safe"
	// "empty" is not in the spec, it is our encoding for requests made with a nil block tag param.
	BlockTagEmpty = "empty"
)

// Errors that might result from decoding parts or the whole of
// an EVM RPC request
var (
	ErrInvalidEthAPIRequest               = errors.New("request is not valid for the eth api")
	ErrUncachaebleEthRequest              = fmt.Errorf("request is not cache-able, current cache-able requests are %s", CacheableEthMethods)
	ErrUncachaebleByBlockNumberEthRequest = fmt.Errorf("request is not cache-able by block number, current cache-able requests by block number are %s or by hash %s", CacheableByBlockNumberMethods, CacheableByBlockHashMethods)
	ErrUncachaebleByBlockHashEthRequest   = fmt.Errorf("request is not cache-able by block hash, current cache-able requests by block hash are  %s", CacheableByBlockHashMethods)
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
	"eth_call",
}

// MethodHasBlockNumberParam returns true when the method expects a block number in the request parameters.
func MethodHasBlockNumberParam(method string) bool {
	var includesBlockNumberParam bool
	for _, cacheableByBlockNumberMethod := range CacheableByBlockNumberMethods {
		if method == cacheableByBlockNumberMethod {
			includesBlockNumberParam = true
			break
		}
	}
	return includesBlockNumberParam
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

// MethodHasBlockHashParam returns true when the method expects a block hash in the request parameters.
func MethodHasBlockHashParam(method string) bool {
	var includesBlockHashParam bool
	for _, cacheableByBlockHashMethod := range CacheableByBlockHashMethods {
		if method == cacheableByBlockHashMethod {
			includesBlockHashParam = true
			break
		}
	}
	return includesBlockHashParam
}

// StaticMethods is a list of static EVM methods which can be cached indefinitely, response will never change.
var StaticMethods = []string{
	"eth_chainId",
	"net_version",
}

// IsMethodStatic checks if method is static. In this context static means that response will never change and can be cached indefinitely.
func IsMethodStatic(method string) bool {
	for _, staticMethod := range StaticMethods {
		if method == staticMethod {
			return true
		}
	}

	return false
}

// CacheableByTxHashMethods is a list of EVM methods which can be cached indefinitely by transaction hash.
// It means that for specific method and params (which includes tx hash) response will never change.
var CacheableByTxHashMethods = []string{
	"eth_getTransactionReceipt",
	"eth_getTransactionByHash",
}

// MethodHasTxHashParam checks if method is cacheable by tx hash.
func MethodHasTxHashParam(method string) bool {
	for _, cacheableByTxHashMethod := range CacheableByTxHashMethods {
		if method == cacheableByTxHashMethod {
			return true
		}
	}

	return false
}

// NoHistoryMethods is a list of JSON-RPC methods that rely only on the present state of the chain.
// They can always be safely routed to an up-to-date pruning cluster.
var NoHistoryMethods = []string{
	"web3_clientVersion",
	"web3_sha3",
	"net_version",
	"net_listening",
	"net_peerCount",
	"eth_protocolVersion",
	"eth_syncing",
	"eth_coinbase",
	"eth_chainId",
	"eth_mining",
	"eth_hashrate",
	"eth_gasPrice",
	"eth_accounts",
	"eth_blockNumber",
	"eth_sign",
	"eth_signTransaction",
	"eth_sendTransaction",
	"eth_sendRawTransaction",
}

// MethodRequiresNoHistory returns true when the JSON-RPC method always functions correctly
// when sent to the latest block.
// This is useful for determining if a request can be routed to a pruning cluster.
func MethodRequiresNoHistory(method string) bool {
	for _, nonHistoricalMethod := range NoHistoryMethods {
		if method == nonHistoricalMethod {
			return true
		}
	}
	return false
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
	"eth_call":                                1,
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
// see https://ethereum.org/en/developers/docs/apis/json-rpc/#default-block
var BlockTagToNumberCodec = map[string]int64{
	BlockTagLatest:    -1,
	BlockTagPending:   -2,
	BlockTagEarliest:  -3,
	BlockTagFinalized: -4,
	BlockTagSafe:      -5,
	// "empty" is not part of the evm json-rpc spec
	// it is our encoding for when no parameter is passed in as a block tag param
	// usually, clients interpret an empty block tag to mean "latest"
	// we track it separately here to more accurately track how users make requests
	BlockTagEmpty: -6,
}

// EVMRPCRequest wraps expected values present in a request
// to the RPC endpoint for an EVM node API
// https://ethereum.org/en/developers/docs/apis/json-rpc/
type EVMRPCRequestEnvelope struct {
	// version of the RPC spec being used
	// https://www.jsonrpc.org/specification
	JSONRPCVersion string `json:"jsonrpc"`
	// ID is a unique id set by the client to match responses to requests
	// it is often an int, but strings are also valid. bools also return valid responses.
	ID     interface{}
	Method string
	Params []interface{}
}

// DecodeEVMRPCRequest attempts to decode the provided bytes into
// an EVMRPCRequestEnvelope for use by the service to extract request details
// and create an enriched request metric, returning the decoded request and error (if any)
func DecodeEVMRPCRequest(body []byte) (*EVMRPCRequestEnvelope, error) {
	var request EVMRPCRequestEnvelope
	err := json.Unmarshal(body, &request)
	return &request, err
}

// DecodeEVMRPCRequest attempts to decode raw bytes to a list of EVMRPCRequestEnvelopes
func DecodeEVMRPCRequestList(body []byte) ([]*EVMRPCRequestEnvelope, error) {
	var request []*EVMRPCRequestEnvelope
	err := json.Unmarshal(body, &request)
	return request, err
}

// ExtractBlockNumberFromEVMRPCRequest attempts to extract the block number
// associated with a request if
// - the request is a valid evm rpc request
// - the method for the request supports specifying a block number
// - the provided block number is a valid tag or number
func (r *EVMRPCRequestEnvelope) ExtractBlockNumberFromEVMRPCRequest(ctx context.Context, blockGetter EVMBlockGetter) (int64, error) {
	// only attempt to extract block number from a valid ethereum api request
	if r.Method == "" {
		return 0, ErrInvalidEthAPIRequest
	}
	// handle cacheable by block number
	if MethodHasBlockNumberParam(r.Method) {
		return ParseBlockNumberFromParams(r.Method, r.Params)
	}
	// handle cacheable by block hash
	if MethodHasBlockHashParam(r.Method) {
		blockNumber, err := lookupBlockNumberFromHashParam(ctx, blockGetter, r.Method, r.Params)
		if err != nil {
			return 0, fmt.Errorf("can't lookup block number from hash param: %v", err)
		}

		return blockNumber, nil
	}
	// handle unable to cached
	return 0, ErrUncachaebleByBlockNumberEthRequest
}

// Generic method to lookup the block number
// based on the hash value in a set of params
func lookupBlockNumberFromHashParam(ctx context.Context, blockGetter EVMBlockGetter, methodName string, params []interface{}) (int64, error) {
	paramIndex, exists := MethodNameToBlockHashParamIndex[methodName]

	if !exists {
		return 0, ErrUncachaebleByBlockHashEthRequest
	}

	blockHash, isString := params[paramIndex].(string)

	if !isString {
		return 0, fmt.Errorf(fmt.Sprintf("error decoding block hash param from params %+v at index %d", params, paramIndex))
	}

	header, err := blockGetter.HeaderByHash(ctx, common.HexToHash(blockHash))
	if err != nil {
		return 0, fmt.Errorf("can't get header by %v block hash: %v", blockHash, err)
	}

	return header.Number.Int64(), nil
}

// Generic method to parse the block number from a set of params
// errors if method does not have a block number in the param, or the param has an unexpected value
// block tags are encoded to an int64 according to the BlockTagToNumberCodec map.
func ParseBlockNumberFromParams(methodName string, params []interface{}) (int64, error) {
	paramIndex, exists := MethodNameToBlockNumberParamIndex[methodName]

	if !exists {
		return 0, ErrUncachaebleByBlockNumberEthRequest
	}

	// capture requests made with empty block tag params
	if params[paramIndex] == nil {
		return BlockTagToNumberCodec["empty"], nil
	}

	tag, isString := params[paramIndex].(string)

	if !isString {
		return 0, fmt.Errorf(fmt.Sprintf("error decoding block number param from params %+v at index %d", params, paramIndex))
	}

	blockNumber, exists := BlockTagToNumberCodec[tag]
	if exists {
		return blockNumber, nil
	}

	return blockParamToInt64(tag)
}

// blockParamToInt64 converts a 0x prefixed base 16 or no-prefixed base 10 string to int64
// and returns an error if value is unable to be converted or out of bounds
func blockParamToInt64(blockParam string) (int64, error) {
	result, valid := cosmosmath.NewIntFromString(blockParam)
	if !valid {
		return 0, fmt.Errorf("unable to parse tag %s to integer", blockParam)
	}

	if !result.IsInt64() {
		return 0, fmt.Errorf("value %s out of range", blockParam)
	}

	return result.Int64(), nil
}

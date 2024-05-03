package decode

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testContext    = context.TODO()
	dummyEthClient = func() *ethclient.Client {
		client := ethclient.Client{}
		return &client
	}()
)

func TestUnitTest_MethodCategorization(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		hasBlockNumber bool
		hasBlockHash   bool
		needsNoHistory bool
	}{
		{
			name:           "block number method",
			method:         "eth_getBlockByNumber",
			hasBlockNumber: true,
			hasBlockHash:   false,
			needsNoHistory: false,
		},
		{
			name:           "block hash method",
			method:         "eth_getBlockByHash",
			hasBlockNumber: false,
			hasBlockHash:   true,
			needsNoHistory: false,
		},
		{
			name:           "needs no history",
			method:         "eth_sendTransaction",
			hasBlockNumber: false,
			hasBlockHash:   false,
			needsNoHistory: true,
		},
		{
			name:           "invalid method",
			method:         "eth_notRealMethod",
			hasBlockNumber: false,
			hasBlockHash:   false,
			needsNoHistory: false,
		},
		{
			name:           "empty method",
			method:         "",
			hasBlockNumber: false,
			hasBlockHash:   false,
			needsNoHistory: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.hasBlockNumber, MethodHasBlockNumberParam(tc.method), "unexpected MethodHasBlockNumberParam result")
			require.Equal(t, tc.hasBlockHash, MethodHasBlockHashParam(tc.method), "unexpected MethodHasBlockHashParam result")
			require.Equal(t, tc.needsNoHistory, MethodRequiresNoHistory(tc.method), "unexpected MethodRequiresNoHistory result")
		})
	}
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestReturnsExpectedBlockForValidRequest(t *testing.T) {
	requestedBlockNumberHexEncoding := "0x2"
	expectedBlockNumber := int64(2)

	validRequest := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByNumber",
		Params: []interface{}{
			requestedBlockNumberHexEncoding, false,
		},
	}

	blockNumber, err := validRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.Nil(t, err)
	assert.Equal(t, expectedBlockNumber, blockNumber)
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestReturnsExpectedBlockNumberForTag(t *testing.T) {
	tags := []string{"latest", "pending", "earliest", "finalized", "safe"}

	for _, requestedBlockTag := range tags {
		validRequest := EVMRPCRequestEnvelope{
			Method: "eth_getBlockByNumber",
			Params: []interface{}{
				requestedBlockTag, false,
			},
		}

		blockNumber, err := validRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

		assert.Nil(t, err)
		assert.Equal(t, BlockTagToNumberCodec[requestedBlockTag], blockNumber)
	}

	// check empty block number
	validRequest := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByNumber",
		Params: []interface{}{
			nil, false,
		},
	}

	blockNumber, err := validRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.Nil(t, err)
	assert.Equal(t, BlockTagToNumberCodec["empty"], blockNumber)
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestFailsForInvalidTag(t *testing.T) {
	requestedBlockTag := "invalid-block-tag"
	validRequest := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByNumber",
		Params: []interface{}{
			requestedBlockTag, false,
		},
	}

	_, err := validRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.ErrorContains(t, err, "unable to parse tag")
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestReturnsErrorWhenRequestMethodEmpty(t *testing.T) {
	invalidRequest := EVMRPCRequestEnvelope{
		Method: "",
	}

	_, err := invalidRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.Equal(t, ErrInvalidEthAPIRequest, err)
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestReturnsErrorWhenInvalidTypeForBlockNumber(t *testing.T) {
	invalidRequest := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByNumber",
		Params: []interface{}{
			false, false,
		},
	}

	_, err := invalidRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.NotNil(t, err)
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestReturnsErrorWhenUnknownRequestMethod(t *testing.T) {
	invalidRequest := EVMRPCRequestEnvelope{
		Method: "eth_web4",
		Params: []interface{}{
			"latest", false,
		},
	}

	_, err := invalidRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.Equal(t, ErrUncachaebleByBlockNumberEthRequest, err)
}

func TestUnitTest_ParseBlockNumberFromParams(t *testing.T) {
	testCases := []struct {
		name                string
		req                 EVMRPCRequestEnvelope
		expectedBlockNumber int64
		expectedErr         string
	}{
		{
			name: "method with block number",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					"0xd", false,
				},
			},
			expectedBlockNumber: 13,
			expectedErr:         "",
		},
		{
			name: "method with block tag",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					"latest", false,
				},
			},
			expectedBlockNumber: BlockTagToNumberCodec[BlockTagLatest],
			expectedErr:         "",
		},
		{
			name: "method with no block number in params",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByHash",
				Params: []interface{}{
					"0xb8d6ffd1ebd2df7a735c72e755886c6dd6587e096ae788558c6f24f31469b271", false,
				},
			},
			expectedBlockNumber: 0,
			expectedErr:         "request is not cache-able by block number",
		},
		{
			name: "method with empty block number",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					nil, false,
				},
			},
			expectedBlockNumber: BlockTagToNumberCodec[BlockTagEmpty],
			expectedErr:         "",
		},
		{
			name: "method with invalid string block number param",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					"not-an-int", false,
				},
			},
			expectedBlockNumber: 0,
			expectedErr:         "unable to parse tag not-an-int to integer",
		},
		{
			name: "method with non-string block number param",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					false, false,
				},
			},
			expectedBlockNumber: 0,
			expectedErr:         "error decoding block number param from params",
		},
		{
			name: "errors on base10 int64 overflow",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					"9223372036854775808", false,
				},
			},
			expectedBlockNumber: 0,
			expectedErr:         "out of range",
		},
		{
			name: "errors on base16 int64 overflow",
			req: EVMRPCRequestEnvelope{
				Method: "eth_getBlockByNumber",
				Params: []interface{}{
					"0x8000000000000000", false,
				},
			},
			expectedBlockNumber: 0,
			expectedErr:         "out of range",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blockNumber, err := ParseBlockNumberFromParams(tc.req.Method, tc.req.Params)
			if tc.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
			}
			require.Equal(t, tc.expectedBlockNumber, blockNumber)
		})
	}
}

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

func TestUnitTest_CacheableParamValidation(t *testing.T) {
	testCases := []struct {
		name           string
		method         string
		hasBlockNumber bool
		hasBlockHash   bool
	}{
		{
			name:           "block number method",
			method:         "eth_getBlockByNumber",
			hasBlockNumber: true,
			hasBlockHash:   false,
		},
		{
			name:           "block hash method",
			method:         "eth_getBlockByHash",
			hasBlockNumber: false,
			hasBlockHash:   true,
		},
		{
			name:           "invalid method",
			method:         "eth_notRealMethod",
			hasBlockNumber: false,
			hasBlockHash:   false,
		},
		{
			name:           "empty method",
			method:         "",
			hasBlockNumber: false,
			hasBlockHash:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.hasBlockNumber, MethodHasBlockNumberParam(tc.method), "unexpected MethodHasBlockNumberParam result")
			require.Equal(t, tc.hasBlockHash, MethodHasBlockHashParam(tc.method), "unexpected MethodHasBlockHashParam result")
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

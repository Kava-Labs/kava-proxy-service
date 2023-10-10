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
	blockNumReq := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByNumber",
		Params: []interface{}{"latest", false},
	}
	require.True(t, blockNumReq.HasBlockNumberParam())
	require.False(t, blockNumReq.HasBlockHashParam())

	blockHashReq := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByHash",
		Params: []interface{}{"0x7d79bac29793ff9b430debd43309875766afaa61e6f49841d33019b1502fea47", false},
	}
	require.True(t, blockHashReq.HasBlockHashParam())
	require.False(t, blockHashReq.HasBlockNumberParam())

	invalidReq := EVMRPCRequestEnvelope{
		Method: "eth_notRealMethod",
		Params: []interface{}{},
	}
	require.False(t, invalidReq.HasBlockNumberParam())
	require.False(t, invalidReq.HasBlockHashParam())

	emptyReq := EVMRPCRequestEnvelope{
		Method: "",
		Params: []interface{}{},
	}
	require.False(t, emptyReq.HasBlockNumberParam())
	require.False(t, emptyReq.HasBlockHashParam())
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

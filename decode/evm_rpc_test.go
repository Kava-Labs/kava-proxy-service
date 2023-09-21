package decode

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
)

var (
	testContext    = context.TODO()
	dummyEthClient = func() *ethclient.Client {
		client := ethclient.Client{}
		return &client
	}()
)

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

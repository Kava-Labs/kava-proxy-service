package decode

import (
	"context"
	"testing"

	cosmosmath "cosmossdk.io/math"
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
	requestBlockNumber, valid := cosmosmath.NewIntFromString(requestedBlockNumberHexEncoding)

	if !valid {
		t.Fatalf("failed to convert %s to cosmos sdk int", requestedBlockNumberHexEncoding)
	}

	validRequest := EVMRPCRequestEnvelope{
		Method: "eth_getBlockByNumber",
		Params: []interface{}{
			requestedBlockNumberHexEncoding, false,
		},
	}

	blockNumber, err := validRequest.ExtractBlockNumberFromEVMRPCRequest(testContext, dummyEthClient)

	assert.Nil(t, err)
	assert.Equal(t, requestBlockNumber, blockNumber)
}

func TestUnitTestExtractBlockNumberFromEVMRPCRequestReturnsExpectedBlockNumberForTag(t *testing.T) {
	requestedBlockTag := "latest"

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

	assert.Equal(t, ErrUncachaebleEthRequest, err)
}

package main_test

import (
	"context"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
)

var (
	proxyServiceURL = os.Getenv("TEST_PROXY_SERVICE_EVM_RPC_URL")
)

func TestE2ETestProxyReturnsNonZeroLatestBlockHeader(t *testing.T) {
	client, err := ethclient.Dial(proxyServiceURL)

	if err != nil {
		t.Fatal(err)
	}

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	assert.Greater(t, int(header.Number.Int64()), 0)
}

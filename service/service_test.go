package service_test

import (
	"context"
	"os"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/assert"
)

var (
	testDefaultContext  = context.TODO()
	proxyServiceURL     = os.Getenv("TEST_PROXY_BACKEND_EVM_RPC_HOST_URL")
	databaseName        = os.Getenv("DATABASE_NAME")
	databaseUsername    = os.Getenv("DATABASE_USERNAME")
	databasePassword    = os.Getenv("DATABASE_PASSWORD")
	databaseEndpointURL = os.Getenv("DATABASE_ENDPOINT_URL")

	dummyConfig = config.Config{
		ProxyBackendHostURL: proxyServiceURL,
		DatabaseName:        databaseName,
		DatabaseUserName:    databaseUsername,
		DatabasePassword:    databasePassword,
		DatabaseEndpointURL: databaseEndpointURL,
	}
	dummyLogger = func() *logging.ServiceLogger {
		logger, err := logging.New("DEBUG")

		if err != nil {
			panic(err)
		}

		return &logger
	}()
)

func TestUnitTestNewWithValidParamsCreatesProxyServiceWithoutError(t *testing.T) {
	_, err := service.New(testDefaultContext, dummyConfig, dummyLogger)

	assert.Nil(t, err)

}

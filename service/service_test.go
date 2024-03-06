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
	testDefaultContext           = context.TODO()
	proxyServiceDefaultURLMapRaw = os.Getenv("TEST_PROXY_BACKEND_HOST_URL_MAP")
	proxyServicePruningURLMapRaw = os.Getenv("TEST_PROXY_PRUNING_BACKEND_HOST_URL_MAP")
	proxyServiceShardURLMapRaw   = os.Getenv("TEST_PROXY_SHARD_BACKEND_HOST_URL_MAP")
	databaseName                 = os.Getenv("DATABASE_NAME")
	databaseUsername             = os.Getenv("DATABASE_USERNAME")
	databasePassword             = os.Getenv("DATABASE_PASSWORD")
	databaseEndpointURL          = os.Getenv("DATABASE_ENDPOINT_URL")
	testServiceLogLevel          = os.Getenv("TEST_SERVICE_LOG_LEVEL")
	evmQueryServiceURL           = os.Getenv("TEST_EVM_QUERY_SERVICE_URL")

	dummyConfig = func() config.Config {
		proxyBackendHostURLMapParsed, err := config.ParseRawProxyBackendHostURLMap(proxyServiceDefaultURLMapRaw)
		if err != nil {
			panic(err)
		}
		proxyPruningBackendHostURLMapParsed, err := config.ParseRawProxyBackendHostURLMap(proxyServicePruningURLMapRaw)
		if err != nil {
			panic(err)
		}
		proxyShardBackendHostURLMapParsed, err := config.ParseRawShardRoutingBackendHostURLMap(proxyServiceShardURLMapRaw)
		if err != nil {
			panic(err)
		}

		conf := config.Config{
			ProxyBackendHostURLMapRaw:        proxyServiceDefaultURLMapRaw,
			ProxyBackendHostURLMapParsed:     proxyBackendHostURLMapParsed,
			ProxyPruningBackendHostURLMapRaw: proxyServicePruningURLMapRaw,
			ProxyPruningBackendHostURLMap:    proxyPruningBackendHostURLMapParsed,
			ProxyShardBackendHostURLMapRaw:   proxyServiceShardURLMapRaw,
			ProxyShardBackendHostURLMap:      proxyShardBackendHostURLMapParsed,

			DatabaseName:        databaseName,
			DatabaseUserName:    databaseUsername,
			DatabasePassword:    databasePassword,
			DatabaseEndpointURL: databaseEndpointURL,
			EvmQueryServiceURL:  evmQueryServiceURL,
		}

		return conf
	}()

	dummyLogger = func() *logging.ServiceLogger {
		logger, err := logging.New(testServiceLogLevel)

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

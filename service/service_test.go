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
	testDefaultContext    = context.TODO()
	proxyServiceURLMapRaw = os.Getenv("TEST_PROXY_BACKEND_HOST_URL_MAP")
	databaseName          = os.Getenv("DATABASE_NAME")
	databaseUsername      = os.Getenv("DATABASE_USERNAME")
	databasePassword      = os.Getenv("DATABASE_PASSWORD")
	databaseEndpointURL   = os.Getenv("DATABASE_ENDPOINT_URL")
	testServiceLogLevel   = os.Getenv("TEST_SERVICE_LOG_LEVEL")

	dummyConfig = func() config.Config {

		proxyBackendHostURLMapParsed, err := config.ParseRawProxyBackendHostURLMap(proxyServiceURLMapRaw)

		if err != nil {
			panic(err)
		}

		conf := config.Config{
			ProxyBackendHostURLMapRaw:    proxyServiceURLMapRaw,
			ProxyBackendHostURLMapParsed: proxyBackendHostURLMapParsed,
			DatabaseName:                 databaseName,
			DatabaseUserName:             databaseUsername,
			DatabasePassword:             databasePassword,
			DatabaseEndpointURL:          databaseEndpointURL,
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

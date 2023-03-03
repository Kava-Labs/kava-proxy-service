package service_test

import (
	"os"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service"
	"github.com/stretchr/testify/assert"
)

var (
	proxyServiceURL = os.Getenv("TEST_PROXY_BACKEND_EVM_RPC_HOST_URL")

	dummyConfig = config.Config{
		ProxyBackendHostURL: proxyServiceURL,
	}
	dummyLogger = &logging.ServiceLogger{}
)

func TestUnitTestNewWithValidParamsCreatesProxyServiceWithoutError(t *testing.T) {
	_, err := service.New(dummyConfig, dummyLogger)

	assert.Nil(t, err)

}

package config_test

import (
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/assert"
)

func init() {
	setDefaultEnv()
}

var (
	defaultConfig = func() config.Config {
		setDefaultEnv()
		return config.ReadConfig()
	}()
)

func TestUnitTestValidateConfigReturnsNilErrorForValidConfig(t *testing.T) {
	err := config.Validate(defaultConfig)

	assert.Nil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidLogLevel(t *testing.T) {
	testConfig := defaultConfig
	testConfig.LogLevel = "whisper"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidProxyBackendHostURL(t *testing.T) {
	testConfig := defaultConfig
	// turns out it's actually very hard to make a non-parseable url 😅
	// https://pkg.go.dev/net/url#Parse
	// > The url may be relative (a path, without a host) or absolute (starting with a scheme). Trying to parse a hostname and path without a scheme is invalid but may not necessarily return an error, due to parsing ambiguities.
	testConfig.ProxyBackendHostURLMapRaw = "kava.com/path%^"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

func TestUnitTestValidateConfigReturnsNoErrorWhenPruningProxyBackendHostURLIsEmpty(t *testing.T) {
	testConfig := defaultConfig
	testConfig.ProxyPruningBackendHostURLMapRaw = ""

	err := config.Validate(testConfig)

	assert.Nil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidProxyBackendHostURLComponents(t *testing.T) {
	testConfig := defaultConfig
	testConfig.ProxyBackendHostURLMapRaw = "localhost:7777,localhost:7778>http://kava:8545$^,localhost:7777>http://kava:8545"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidProxyPruningBackendHostURLComponents(t *testing.T) {
	testConfig := defaultConfig
	testConfig.ProxyPruningBackendHostURLMapRaw = "localhost:7777,localhost:7778>http://kava:8545$^,localhost:7777>http://kava:8545"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidProxyServicePort(t *testing.T) {
	testConfig := defaultConfig
	testConfig.ProxyServicePort = "abc"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidMetricPartitioningPrefillPeriodDays(t *testing.T) {
	testConfig := defaultConfig
	testConfig.MetricPartitioningPrefillPeriodDays = config.MaxMetricPartitioningPrefillPeriodDays + 1

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

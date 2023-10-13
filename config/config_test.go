package config_test

import (
	"os"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/assert"
)

var (
	proxyServicePort                     = "7777"
	randomEnvironmentVariableKey         = "TEST_KAVA_RANDOM_VALUE"
	proxyServiceBackendHostURLMap        = os.Getenv("TEST_PROXY_BACKEND_HOST_URL_MAP")
	proxyServiceHeightBasedRouting       = os.Getenv("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED")
	proxyServicePruningBackendHostURLMap = os.Getenv("TEST_PROXY_PRUNING_BACKEND_HOST_URL_MAP")
)

func TestUnitTestEnvODefaultReturnsDefaultIfEnvironmentVariableNotSet(t *testing.T) {
	err := os.Unsetenv(randomEnvironmentVariableKey)

	assert.Nil(t, err, "error clearing environment variable")

	defaultValue := "default"

	value := config.EnvOrDefault(randomEnvironmentVariableKey, defaultValue)

	assert.Equal(t, defaultValue, value)
}

func TestUnitTestEnvODefaultReturnsSetValue(t *testing.T) {
	setValue := "default"
	err := os.Setenv(randomEnvironmentVariableKey, setValue)

	assert.Nil(t, err, "error settting environment variable")

	value := config.EnvOrDefault(randomEnvironmentVariableKey, "")

	assert.Equal(t, setValue, value)
}

func TestUnitTestReadConfigReturnsConfigWithValuesFromEnv(t *testing.T) {
	setDefaultEnv()

	readConfig := config.ReadConfig()

	assert.Equal(t, config.DEFAULT_LOG_LEVEL, readConfig.LogLevel)
	assert.Equal(t, proxyServicePort, readConfig.ProxyServicePort)
}

func TestUnitTestParseHostMapReturnsErrEmptyHostMapWhenEmpty(t *testing.T) {
	_, err := config.ParseRawProxyBackendHostURLMap("")
	assert.ErrorIs(t, err, config.ErrEmptyHostMap)
}

func setDefaultEnv() {
	os.Setenv(config.PROXY_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, proxyServiceBackendHostURLMap)
	os.Setenv(config.PROXY_HEIGHT_BASED_ROUTING_ENABLED_KEY, proxyServiceHeightBasedRouting)
	os.Setenv(config.PROXY_PRUNING_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, proxyServicePruningBackendHostURLMap)
	os.Setenv(config.PROXY_SERVICE_PORT_ENVIRONMENT_KEY, proxyServicePort)
	os.Setenv(config.LOG_LEVEL_ENVIRONMENT_KEY, config.DEFAULT_LOG_LEVEL)
}

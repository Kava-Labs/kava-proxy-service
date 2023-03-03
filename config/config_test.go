package config_test

import (
	"os"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/assert"
)

var (
	proxyServicePort             = "7777"
	randomEnvironmentVariableKey = "TEST_KAVA_RANDOM_VALUE"
	proxyServiceURL              = os.Getenv("TEST_PROXY_BACKEND_EVM_RPC_HOST_URL")
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
	assert.Equal(t, proxyServiceURL, readConfig.ProxyBackendHostURL)
	assert.Equal(t, proxyServicePort, readConfig.ProxyServicePort)
}

func setDefaultEnv() {
	os.Setenv(config.PROXY_BACKEND_HOST_URL_ENVIRONMENT_KEY, proxyServiceURL)
	os.Setenv(config.PROXY_SERVICE_PORT_ENVIRONMENT_KEY, proxyServicePort)
	os.Setenv(config.LOG_LEVEL_ENVIRONMENT_KEY, config.DEFAULT_LOG_LEVEL)
}

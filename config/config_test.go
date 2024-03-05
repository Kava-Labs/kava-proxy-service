package config_test

import (
	"net/url"
	"os"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	proxyServicePort                     = "7777"
	randomEnvironmentVariableKey         = "TEST_KAVA_RANDOM_VALUE"
	proxyServiceBackendHostURLMap        = os.Getenv("TEST_PROXY_BACKEND_HOST_URL_MAP")
	proxyServiceHeightBasedRouting       = os.Getenv("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED")
	proxyServicePruningBackendHostURLMap = os.Getenv("TEST_PROXY_PRUNING_BACKEND_HOST_URL_MAP")
	proxyServiceShardedRoutingEnabled    = os.Getenv("TEST_PROXY_HEIGHT_BASED_ROUTING_ENABLED")
	proxyServiceShardBackendHostURLMap   = os.Getenv("TEST_PROXY_SHARD_BACKEND_HOST_URL_MAP")
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

func TestUnitTestParseRawShardRoutingBackendHostURLMap(t *testing.T) {
	parsed, err := config.ParseRawShardRoutingBackendHostURLMap("localhost:7777>10|http://kava-shard-10:8545|20|http://kava-shard-20:8545")
	require.NoError(t, err)
	expected := map[string]config.IntervalURLMap{
		"localhost:7777": config.NewIntervalURLMap(map[uint64]*url.URL{
			10: mustUrl("http://kava-shard-10:8545"),
			20: mustUrl("http://kava-shard-20:8545"),
		}),
	}
	require.Equal(t, expected, parsed)

	_, err = config.ParseRawShardRoutingBackendHostURLMap("no-shard-def")
	require.ErrorContains(t, err, "expected shard definition like <host>:<end-height>|<backend-route>")

	_, err = config.ParseRawShardRoutingBackendHostURLMap("invalid-shard-def>odd|number|bad")
	require.ErrorContains(t, err, "unexpected <end-height>|<backend-route> sequence for invalid-shard-def")

	_, err = config.ParseRawShardRoutingBackendHostURLMap("invalid-height>NaN|backend-host")
	require.ErrorContains(t, err, "invalid shard end height (NaN) for host invalid-height")

	_, err = config.ParseRawShardRoutingBackendHostURLMap("invalid-backend-host>100|")
	require.ErrorContains(t, err, "invalid shard backend route () for height 100 of host invalid-backend-host")

	_, err = config.ParseRawShardRoutingBackendHostURLMap("unsorted-shards>100|backend-100|50|backend-50")
	require.ErrorContains(t, err, "shard map expects end blocks to be ordered")

	_, err = config.ParseRawShardRoutingBackendHostURLMap("multiple-shards-for-same-height>10|magic|20|dino|20|dinosaur")
	require.ErrorContains(t, err, "multiple shards defined for multiple-shards-for-same-height with end block 20")
}

func setDefaultEnv() {
	os.Setenv(config.PROXY_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, proxyServiceBackendHostURLMap)
	os.Setenv(config.PROXY_HEIGHT_BASED_ROUTING_ENABLED_KEY, proxyServiceHeightBasedRouting)
	os.Setenv(config.PROXY_PRUNING_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, proxyServicePruningBackendHostURLMap)
	os.Setenv(config.PROXY_SHARDED_ROUTING_ENABLED_ENVIRONMENT_KEY, proxyServiceShardedRoutingEnabled)
	os.Setenv(config.PROXY_SHARD_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, proxyServiceShardBackendHostURLMap)
	os.Setenv(config.PROXY_SERVICE_PORT_ENVIRONMENT_KEY, proxyServicePort)
	os.Setenv(config.LOG_LEVEL_ENVIRONMENT_KEY, config.DEFAULT_LOG_LEVEL)
}

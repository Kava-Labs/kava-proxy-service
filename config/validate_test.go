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
	testConfig.ProxyBackendHostURL = "kava.com/path%^"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

func TestUnitTestValidateConfigReturnsErrorIfInvalidProxyServicePort(t *testing.T) {
	testConfig := defaultConfig
	testConfig.ProxyServicePort = "abc"

	err := config.Validate(testConfig)

	assert.NotNil(t, err)
}

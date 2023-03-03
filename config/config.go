// package config provides functions and values
// for reading and validating kava proxy service configuration
package config

import (
	"net/url"
	"os"
)

type Config struct {
	ProxyServicePort          string
	LogLevel                  string
	ProxyBackendHostURL       string
	ProxyBackendHostURLParsed url.URL
}

const (
	LOG_LEVEL_ENVIRONMENT_KEY              = "LOG_LEVEL"
	DEFAULT_LOG_LEVEL                      = "INFO"
	PROXY_BACKEND_HOST_URL_ENVIRONMENT_KEY = "PROXY_BACKEND_HOST_URL"
	PROXY_SERVICE_PORT_ENVIRONMENT_KEY     = "PROXY_SERVICE_PORT"
)

// EnvOrDefault fetches an environment variable value, or if not set returns the fallback value
func EnvOrDefault(key string, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// ReadConfig attempts to parse service config from environment values
// the returned config may be invalid and should be validated via the `Validate`
// function of the Config package before use
func ReadConfig() Config {
	rawProxyBackendHostURL := os.Getenv(PROXY_BACKEND_HOST_URL_ENVIRONMENT_KEY)
	// best effort to pares, callers are responsible for validating
	// before using any values read
	parsedProxyBackendHostURL, _ := url.Parse(rawProxyBackendHostURL)

	return Config{
		ProxyServicePort:          os.Getenv(PROXY_SERVICE_PORT_ENVIRONMENT_KEY),
		LogLevel:                  EnvOrDefault(LOG_LEVEL_ENVIRONMENT_KEY, DEFAULT_LOG_LEVEL),
		ProxyBackendHostURL:       rawProxyBackendHostURL,
		ProxyBackendHostURLParsed: *parsedProxyBackendHostURL,
	}
}

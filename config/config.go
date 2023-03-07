// package config provides functions and values
// for reading and validating kava proxy service configuration
package config

import (
	"net/url"
	"os"
	"strconv"
)

type Config struct {
	ProxyServicePort            string
	LogLevel                    string
	ProxyBackendHostURL         string
	ProxyBackendHostURLParsed   url.URL
	DatabaseName                string
	DatabaseEndpointURL         string
	DatabaseUserName            string
	DatabasePassword            string
	DatabaseSSLEnabled          bool
	DatabaseQueryLoggingEnabled bool
}

const (
	LOG_LEVEL_ENVIRONMENT_KEY                      = "LOG_LEVEL"
	DEFAULT_LOG_LEVEL                              = "INFO"
	PROXY_BACKEND_HOST_URL_ENVIRONMENT_KEY         = "PROXY_BACKEND_HOST_URL"
	PROXY_SERVICE_PORT_ENVIRONMENT_KEY             = "PROXY_SERVICE_PORT"
	DATABASE_NAME_ENVIRONMENT_KEY                  = "DATABASE_NAME"
	DATABASE_ENDPOINT_URL_ENVIRONMENT_KEY          = "DATABASE_ENDPOINT_URL"
	DATABASE_USERNAME_ENVIRONMENT_KEY              = "DATABASE_USERNAME"
	DATABASE_PASSWORD_ENVIRONMENT_KEY              = "DATABASE_PASSWORD"
	DATABASE_SSL_ENABLED_ENVIRONMENT_KEY           = "DATABASE_SSL_ENABLED"
	DATABASE_QUERY_LOGGING_ENABLED_ENVIRONMENT_KEY = "DATABASE_QUERY_LOGGING_ENABLED"
)

// EnvOrDefault fetches an environment variable value, or if not set returns the fallback value
func EnvOrDefault(key string, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

// EnvOrDefaultBool fetches a boolean environment variable value, or if not set returns the fallback value
func EnvOrDefaultBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		val, err := strconv.ParseBool(val)
		if err != nil {
			return fallback
		}
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
		ProxyServicePort:            os.Getenv(PROXY_SERVICE_PORT_ENVIRONMENT_KEY),
		LogLevel:                    EnvOrDefault(LOG_LEVEL_ENVIRONMENT_KEY, DEFAULT_LOG_LEVEL),
		ProxyBackendHostURL:         rawProxyBackendHostURL,
		ProxyBackendHostURLParsed:   *parsedProxyBackendHostURL,
		DatabaseName:                os.Getenv(DATABASE_NAME_ENVIRONMENT_KEY),
		DatabaseEndpointURL:         os.Getenv(DATABASE_ENDPOINT_URL_ENVIRONMENT_KEY),
		DatabaseUserName:            os.Getenv(DATABASE_USERNAME_ENVIRONMENT_KEY),
		DatabasePassword:            os.Getenv(DATABASE_PASSWORD_ENVIRONMENT_KEY),
		DatabaseSSLEnabled:          EnvOrDefaultBool(DATABASE_SSL_ENABLED_ENVIRONMENT_KEY, false),
		DatabaseQueryLoggingEnabled: EnvOrDefaultBool(DATABASE_QUERY_LOGGING_ENABLED_ENVIRONMENT_KEY, true),
	}
}

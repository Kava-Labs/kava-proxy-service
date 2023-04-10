// package config provides functions and values
// for reading and validating kava proxy service configuration
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ProxyServicePort                string
	LogLevel                        string
	ProxyBackendHostURLMapRaw       string
	ProxyBackendHostURLMapParsed    map[string]url.URL
	DatabaseName                    string
	DatabaseEndpointURL             string
	DatabaseUserName                string
	DatabasePassword                string
	DatabaseSSLEnabled              bool
	DatabaseQueryLoggingEnabled     bool
	RunDatabaseMigrations           bool
	HTTPReadTimeoutSeconds          int64
	HTTPWriteTimeoutSeconds         int64
	MetricCompactionRoutineInterval time.Duration
}

const (
	LOG_LEVEL_ENVIRONMENT_KEY                      = "LOG_LEVEL"
	DEFAULT_LOG_LEVEL                              = "INFO"
	PROXY_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY     = "PROXY_BACKEND_HOST_URL_MAP"
	PROXY_SERVICE_PORT_ENVIRONMENT_KEY             = "PROXY_SERVICE_PORT"
	DATABASE_NAME_ENVIRONMENT_KEY                  = "DATABASE_NAME"
	DATABASE_ENDPOINT_URL_ENVIRONMENT_KEY          = "DATABASE_ENDPOINT_URL"
	DATABASE_USERNAME_ENVIRONMENT_KEY              = "DATABASE_USERNAME"
	DATABASE_PASSWORD_ENVIRONMENT_KEY              = "DATABASE_PASSWORD"
	DATABASE_SSL_ENABLED_ENVIRONMENT_KEY           = "DATABASE_SSL_ENABLED"
	DATABASE_QUERY_LOGGING_ENABLED_ENVIRONMENT_KEY = "DATABASE_QUERY_LOGGING_ENABLED"
	RUN_DATABASE_MIGRATIONS_ENVIRONMENT_KEY        = "RUN_DATABASE_MIGRATIONS"
	DEFAULT_HTTP_READ_TIMEOUT                      = 30
	DEFAULT_HTTP_WRITE_TIMEOUT                     = 60
	HTTP_READ_TIMEOUT_ENVIRONMENT_KEY              = "HTTP_READ_TIMEOUT_SECONDS"
	HTTP_WRITE_TIMEOUT_ENVIRONMENT_KEY             = "HTTP_WRITE_TIMEOUT_SECONDS"
	METRIC_COMPACTION_ROUTINE_INTERVAL_KEY         = "METRIC_COMPACTION_ROUTINE_INTERVAL_SECONDS"
	// 60 seconds / minute * 60 minutes = 1 hour
	DEFAULT_METRIC_COMPACTION_ROUTINE_INTERVAL_SECONDS = 3600
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

// EnvOrDefaultInt64 fetches an int64 environment variable value, or if not set returns the fallback value
func EnvOrDefaultInt64(key string, fallback int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		val, err := strconv.ParseInt(val, 0, 64)
		if err != nil {
			return fallback
		}
		return val
	}
	return fallback
}

// EnvOrDefaultInt fetches an int environment variable value, or if not set returns the fallback value
func EnvOrDefaultInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		val, err := strconv.Atoi(val)
		if err != nil {
			return fallback
		}
		return val
	}
	return fallback
}

// seperator for a single entry mapping the <host to proxy for> to
// <backend server for host>
const PROXY_BACKEND_HOST_URL_MAP_ENTRY_DELIMITER = ","

// seperator for
const PROXY_BACKEND_HOST_URL_MAP_SUB_COMPONENT_DELIMITER = ">"

// ParseRawProxyBackendHostURLMap attempts to parse mappings
// of hostname to proxy for and the backend servers to proxy
// the request to, returning the mapping and error (if any).
func ParseRawProxyBackendHostURLMap(raw string) (map[string]url.URL, error) {
	hostURLMap := map[string]url.URL{}
	var combinedErr error

	entries := strings.Split(raw, PROXY_BACKEND_HOST_URL_MAP_ENTRY_DELIMITER)

	if len(entries) < 1 {
		return hostURLMap, fmt.Errorf("found zero mappings delimited by %s in %s", PROXY_BACKEND_HOST_URL_MAP_ENTRY_DELIMITER, raw)
	}

	for _, entry := range entries {
		entryComponents := strings.Split(entry, PROXY_BACKEND_HOST_URL_MAP_SUB_COMPONENT_DELIMITER)

		if len(entryComponents) != 2 {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("expected map value of host to backend url delimited by %s, got %s", PROXY_BACKEND_HOST_URL_MAP_SUB_COMPONENT_DELIMITER, entry))

			continue
		}

		host := entryComponents[0]
		rawBackendURL := entryComponents[1]
		parsedBackendURL, err := url.Parse(rawBackendURL)

		if err != nil {
			combinedErr = errors.Join(combinedErr, fmt.Errorf("expected map value of host to backend url delimited by %s, got %s", PROXY_BACKEND_HOST_URL_MAP_SUB_COMPONENT_DELIMITER, entry))

			continue
		}

		hostURLMap[host] = *parsedBackendURL
	}

	return hostURLMap, combinedErr
}

// ReadConfig attempts to parse service config from environment values
// the returned config may be invalid and should be validated via the `Validate`
// function of the Config package before use
func ReadConfig() Config {
	rawProxyBackendHostURLMap := os.Getenv(PROXY_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY)
	// best effort to pares, callers are responsible for validating
	// before using any values read
	parsedProxyBackendHostURLMap, _ := ParseRawProxyBackendHostURLMap(rawProxyBackendHostURLMap)

	return Config{
		ProxyServicePort:                os.Getenv(PROXY_SERVICE_PORT_ENVIRONMENT_KEY),
		LogLevel:                        EnvOrDefault(LOG_LEVEL_ENVIRONMENT_KEY, DEFAULT_LOG_LEVEL),
		ProxyBackendHostURLMapRaw:       rawProxyBackendHostURLMap,
		ProxyBackendHostURLMapParsed:    parsedProxyBackendHostURLMap,
		DatabaseName:                    os.Getenv(DATABASE_NAME_ENVIRONMENT_KEY),
		DatabaseEndpointURL:             os.Getenv(DATABASE_ENDPOINT_URL_ENVIRONMENT_KEY),
		DatabaseUserName:                os.Getenv(DATABASE_USERNAME_ENVIRONMENT_KEY),
		DatabasePassword:                os.Getenv(DATABASE_PASSWORD_ENVIRONMENT_KEY),
		DatabaseSSLEnabled:              EnvOrDefaultBool(DATABASE_SSL_ENABLED_ENVIRONMENT_KEY, false),
		DatabaseQueryLoggingEnabled:     EnvOrDefaultBool(DATABASE_QUERY_LOGGING_ENABLED_ENVIRONMENT_KEY, true),
		RunDatabaseMigrations:           EnvOrDefaultBool(RUN_DATABASE_MIGRATIONS_ENVIRONMENT_KEY, false),
		HTTPReadTimeoutSeconds:          EnvOrDefaultInt64(HTTP_READ_TIMEOUT_ENVIRONMENT_KEY, DEFAULT_HTTP_READ_TIMEOUT),
		HTTPWriteTimeoutSeconds:         EnvOrDefaultInt64(HTTP_WRITE_TIMEOUT_ENVIRONMENT_KEY, DEFAULT_HTTP_WRITE_TIMEOUT),
		MetricCompactionRoutineInterval: time.Duration(time.Duration(EnvOrDefaultInt(METRIC_COMPACTION_ROUTINE_INTERVAL_KEY, DEFAULT_METRIC_COMPACTION_ROUTINE_INTERVAL_SECONDS)) * time.Second),
	}
}

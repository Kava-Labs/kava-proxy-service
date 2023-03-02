// package config provides functions and values
// for reading and validating kava proxy service configuration
package config

import "os"

type Config struct {
	LogLevel string
}

const (
	LOG_LEVEL_ENVIRONMENT_KEY = "LOG_LEVEL"
	DEFAULT_LOG_LEVEL         = "INFO"
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
	return Config{
		LogLevel: EnvOrDefault(LOG_LEVEL_ENVIRONMENT_KEY, DEFAULT_LOG_LEVEL),
	}
}

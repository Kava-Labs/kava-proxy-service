package config

import (
	"errors"
	"fmt"
)

var (
	ValidLogLevels = [3]string{"DEBUG", "INFO", "ERROR"}
)

// Validate validates the provided config
// returning a list of errors that can be unwrapped with `errors.Unwrap`
// or nil if the config is valid
func Validate(config Config) error {
	var validLogLevel bool
	var err error

	for _, validLevel := range ValidLogLevels {
		if config.LogLevel == validLevel {
			validLogLevel = true
			break
		}
	}

	if !validLogLevel {
		err = errors.Join(err, fmt.Errorf("invalid LOG_LEVEL specified %s, supported values are %v", config.LogLevel, ValidLogLevels))
	}

	return err
}

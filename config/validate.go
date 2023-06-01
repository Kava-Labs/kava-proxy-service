package config

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	ValidLogLevels = [4]string{"TRACE", "DEBUG", "INFO", "ERROR"}
	// restrict to max 1 month to guarantee constraint that
	// metric partitioning routine never needs to create partitions
	// spanning more than 2 calendar months
	MaxMetricPartitioningPrefillPeriodDays = 28
)

// Validate validates the provided config
// returning a list of errors that can be unwrapped with `errors.Unwrap`
// or nil if the config is valid
func Validate(config Config) error {
	var validLogLevel bool
	var allErrs error

	for _, validLevel := range ValidLogLevels {
		if config.LogLevel == validLevel {
			validLogLevel = true
			break
		}
	}

	if !validLogLevel {
		allErrs = fmt.Errorf("invalid %s specified %s, supported values are %v", LOG_LEVEL_ENVIRONMENT_KEY, config.LogLevel, ValidLogLevels)
	}

	_, err := ParseRawProxyBackendHostURLMap(config.ProxyBackendHostURLMapRaw)

	if err != nil {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s", PROXY_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, config.ProxyBackendHostURLMapRaw))
	}

	_, err = strconv.Atoi(config.ProxyServicePort)

	if err != nil {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s", PROXY_SERVICE_PORT_ENVIRONMENT_KEY, config.ProxyServicePort))
	}

	if config.MetricPartitioningPrefillPeriodDays > MaxMetricPartitioningPrefillPeriodDays || config.MetricPartitioningPrefillPeriodDays < 1 {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %d, must be non-zero and less than %d", METRIC_PARTITIONING_PREFILL_PERIOD_DAYS_ENVIRONMENT_KEY, config.MetricPartitioningPrefillPeriodDays, MaxMetricPartitioningPrefillPeriodDays))
	}

	return allErrs
}

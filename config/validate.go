package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
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
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %d, must be non-zero and less than or equal to %d", METRIC_PARTITIONING_PREFILL_PERIOD_DAYS_ENVIRONMENT_KEY, config.MetricPartitioningPrefillPeriodDays, MaxMetricPartitioningPrefillPeriodDays))
	}

	if config.RedisEndpointURL == "" {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s, must not be empty", REDIS_ENDPOINT_URL_ENVIRONMENT_KEY, config.RedisEndpointURL))
	}
	if config.CacheTTL <= 0 {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s, must be greater than zero", CACHE_TTL_ENVIRONMENT_KEY, config.CacheTTL))
	}
	if strings.Contains(config.CachePrefix, ":") {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s, must not contain colon symbol", CACHE_PREFIX_ENVIRONMENT_KEY, config.CachePrefix))
	}
	if config.CachePrefix == "" {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s, must not be empty", CACHE_PREFIX_ENVIRONMENT_KEY, config.CachePrefix))
	}

	return allErrs
}

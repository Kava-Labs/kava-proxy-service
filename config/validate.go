package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
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
	var err error

	for _, validLevel := range ValidLogLevels {
		if config.LogLevel == validLevel {
			validLogLevel = true
			break
		}
	}

	if !validLogLevel {
		allErrs = fmt.Errorf("invalid %s specified %s, supported values are %v", LOG_LEVEL_ENVIRONMENT_KEY, config.LogLevel, ValidLogLevels)
	}

	if err = validateHostURLMap(config.ProxyBackendHostURLMapRaw, false); err != nil {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s", PROXY_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, config.ProxyBackendHostURLMapRaw), err)
	}

	if err = validateHostURLMap(config.ProxyPruningBackendHostURLMapRaw, true); err != nil {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s", PROXY_PRUNING_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY, config.ProxyPruningBackendHostURLMapRaw), err)
	}

	if err = validateDefaultHostMapContainsHosts(
		PROXY_PRUNING_BACKEND_HOST_URL_MAP_ENVIRONMENT_KEY,
		config.ProxyBackendHostURLMapParsed,
		config.ProxyPruningBackendHostURLMap,
	); err != nil {
		allErrs = errors.Join(allErrs, err)
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

	if err := checkTTLConfig(config.CacheIndefinitely, config.CacheTTL, CACHE_INDEFINITELY_ENVIRONMENT_KEY, CACHE_TTL_ENVIRONMENT_KEY); err != nil {
		allErrs = errors.Join(allErrs, err)
	}
	if err := checkTTLConfig(config.CacheMethodHasBlockNumberParamIndefinitely, config.CacheMethodHasBlockNumberParamTTL, CACHE_METHOD_HAS_BLOCK_NUMBER_PARAM_INDEFINITELY_ENVIRONMENT_KEY, CACHE_METHOD_HAS_BLOCK_NUMBER_PARAM_TTL_ENVIRONMENT_KEY); err != nil {
		allErrs = errors.Join(allErrs, err)
	}
	if err := checkTTLConfig(config.CacheMethodHasBlockHashParamIndefinitely, config.CacheMethodHasBlockHashParamTTL, CACHE_METHOD_HAS_BLOCK_HASH_PARAM_INDEFINITELY_ENVIRONMENT_KEY, CACHE_METHOD_HAS_BLOCK_HASH_PARAM_TTL_ENVIRONMENT_KEY); err != nil {
		allErrs = errors.Join(allErrs, err)
	}
	if err := checkTTLConfig(config.CacheStaticMethodIndefinitely, config.CacheStaticMethodTTL, CACHE_STATIC_METHOD_INDEFINITELY_ENVIRONMENT_KEY, CACHE_STATIC_METHOD_TTL_ENVIRONMENT_KEY); err != nil {
		allErrs = errors.Join(allErrs, err)
	}

	if strings.Contains(config.CachePrefix, ":") {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s, must not contain colon symbol", CACHE_PREFIX_ENVIRONMENT_KEY, config.CachePrefix))
	}
	if config.CachePrefix == "" {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s, must not be empty", CACHE_PREFIX_ENVIRONMENT_KEY, config.CachePrefix))
	}

	if err = validateHostnameToHeaderValueMap(config.HostnameToAccessControlAllowOriginValueMapRaw, true); err != nil {
		allErrs = errors.Join(allErrs, fmt.Errorf("invalid %s specified %s", HOSTNAME_TO_ACCESS_CONTROL_ALLOW_ORIGIN_VALUE_MAP_ENVIRONMENT_KEY, config.HostnameToAccessControlAllowOriginValueMapRaw), err)
	}

	return allErrs
}

func checkTTLConfig(cacheIndefinitely bool, cacheTTL time.Duration, cacheIndefinitelyKey, cacheTTLKey string) error {
	if !cacheIndefinitely && cacheTTL <= 0 {
		return fmt.Errorf("invalid %s specified %s, must be greater than zero (when %v is false)", cacheTTLKey, cacheTTL, cacheIndefinitelyKey)
	}
	if cacheIndefinitely && cacheTTL != 0 {
		return fmt.Errorf("invalid %s specified %s, must be zero (when %v is true)", cacheTTLKey, cacheTTL, cacheIndefinitelyKey)
	}

	return nil
}

// validateHostURLMap validates a raw backend host URL map, optionally allowing the map to be empty
func validateHostURLMap(raw string, allowEmpty bool) error {
	_, err := ParseRawProxyBackendHostURLMap(raw)
	if allowEmpty && errors.Is(err, ErrEmptyHostMap) {
		err = nil
	}
	return err
}

// validateHostnameToHeaderValueMap validates a raw hostname to header value map, optionally allowing the map to be empty
func validateHostnameToHeaderValueMap(raw string, allowEmpty bool) error {
	_, err := ParseRawHostnameToHeaderValueMap(raw)
	if allowEmpty && errors.Is(err, ErrEmptyHostnameToHeaderValueMap) {
		err = nil
	}
	return err
}

// validateDefaultHostMapContainsHosts returns an error if there are hosts in hostMap that
// are not in defaultHostMap
// example: hosts in the pruning map should always have a default fallback backend
func validateDefaultHostMapContainsHosts(mapName string, defaultHostsMap, hostsMap map[string]url.URL) error {
	for host := range hostsMap {
		if _, found := defaultHostsMap[host]; !found {
			return fmt.Errorf("host %s is in %s but not in default host map", host, mapName)
		}
	}
	return nil
}

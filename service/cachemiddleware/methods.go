package cachemiddleware

// CacheableMethodSubset is a subset of methods that are cacheable. This allows
// for a more granular approach to which methods to cache.
var CacheableMethodSubset = map[string]bool{
	"eth_blockNumber": true,
}

// InCacheableMethodSubset returns true if the method is in the cacheable
// method subset.
func InCacheableMethodSubset(method string) bool {
	return CacheableMethodSubset[method]
}

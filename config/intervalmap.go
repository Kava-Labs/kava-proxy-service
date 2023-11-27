package config

import (
	"net/url"
	"sort"
)

// IntervalURLMap stores URLs associated with a range of numbers.
// The intervals are defined by their endpoints and must not overlap.
// The intervals are exclusive of the endpoints.
type IntervalURLMap struct {
	valueByEndpoint map[uint64]*url.URL
	endpoints       []uint64
}

// NewIntervalURLMap creates a new IntervalMap from a map of interval endpoint => url.
// The intervals are exclusive of their endpoint.
// ie. if the lowest value endpoint in the map is 10, the interval is for all numbers 1 through 9.
func NewIntervalURLMap(valueByEndpoint map[uint64]*url.URL) IntervalURLMap {
	endpoints := make([]uint64, 0, len(valueByEndpoint))
	for e := range valueByEndpoint {
		endpoints = append(endpoints, e)
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i] < endpoints[j] })

	return IntervalURLMap{
		valueByEndpoint: valueByEndpoint,
		endpoints:       endpoints,
	}
}

// Lookup finds the value associated with the interval containing the number, if it exists.
func (im *IntervalURLMap) Lookup(num uint64) (*url.URL, bool) {
	i := sort.Search(len(im.endpoints), func(i int) bool { return im.endpoints[i] > num })

	if i < len(im.endpoints) && num < im.endpoints[i] {
		return im.valueByEndpoint[im.endpoints[i]], true
	}

	return nil, false
}

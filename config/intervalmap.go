package config

import (
	"net/url"
	"sort"
)

// IntervalURLMap stores URLs associated with a range of numbers.
// The intervals are defined by their endpoints and must not overlap.
// The intervals are inclusive of the endpoints.
type IntervalURLMap struct {
	UrlByEndHeight map[uint64]*url.URL
	endpoints      []uint64
}

// NewIntervalURLMap creates a new IntervalMap from a map of interval endpoint => url.
// The intervals are inclusive of their endpoint.
// ie. if the lowest value endpoint in the map is 10, the interval is for all numbers 1 through 10.
func NewIntervalURLMap(urlByEndHeight map[uint64]*url.URL) IntervalURLMap {
	endpoints := make([]uint64, 0, len(urlByEndHeight))
	for e := range urlByEndHeight {
		endpoints = append(endpoints, e)
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i] < endpoints[j] })

	return IntervalURLMap{
		UrlByEndHeight: urlByEndHeight,
		endpoints:      endpoints,
	}
}

// Lookup finds the value associated with the interval containing the number, if it exists.
func (im *IntervalURLMap) Lookup(num uint64) (*url.URL, uint64, bool) {
	i := sort.Search(len(im.endpoints), func(i int) bool { return im.endpoints[i] >= num })

	if i < len(im.endpoints) && num <= im.endpoints[i] {
		return im.UrlByEndHeight[im.endpoints[i]], im.endpoints[i], true
	}

	return nil, 0, false
}

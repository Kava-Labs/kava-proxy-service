package batchmdw

import (
	"testing"

	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
	"github.com/stretchr/testify/require"
)

func TestUnitTest_cacheHitValue(t *testing.T) {
	for _, tc := range []struct {
		name     string
		total    int
		hits     int
		expected string
	}{
		{
			name:     "no hits => MISS",
			total:    5,
			hits:     0,
			expected: cachemdw.CacheMissHeaderValue,
		},
		{
			name:     "all hits => HIT",
			total:    5,
			hits:     5,
			expected: cachemdw.CacheHitHeaderValue,
		},
		{
			name:     "some hits => PARTIAL",
			total:    5,
			hits:     3,
			expected: cachemdw.CachePartialHeaderValue,
		},
		{
			name:     "invalid 0 case => MISS",
			total:    0,
			hits:     0,
			expected: cachemdw.CacheMissHeaderValue,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := cacheHitValue(tc.total, tc.hits)
			require.Equal(t, tc.expected, actual)
		})
	}
}

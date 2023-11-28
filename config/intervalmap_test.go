package config_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/stretchr/testify/require"
)

func mustUrl(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(fmt.Sprintf("failed to parse url %s: %s", s, err))
	}
	return u
}

func TestUnitTestIntervalMap(t *testing.T) {
	valueByEndpoint := map[uint64]*url.URL{
		10:  mustUrl("A"),
		20:  mustUrl("B"),
		100: mustUrl("C"),
	}
	intervalmap := config.NewIntervalURLMap(valueByEndpoint)

	testCases := []struct {
		value           uint64
		expectFound     bool
		expectEndHeight uint64
		expectResult    string
	}{
		{1, true, 10, "A"},
		{9, true, 10, "A"},
		{10, true, 20, "B"},
		{15, true, 20, "B"},
		{20, true, 100, "C"},
		{75, true, 100, "C"},
		{100, false, 0, ""},
		{300, false, 0, ""},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Lookup(%d)", tc.value), func(t *testing.T) {
			result, endHeight, found := intervalmap.Lookup(tc.value)
			require.Equal(t, tc.expectFound, found)
			require.Equal(t, tc.expectEndHeight, endHeight)
			if tc.expectResult == "" {
				require.Nil(t, result)
			} else {
				require.Equal(t, tc.expectResult, result.String())
			}
		})
	}
}

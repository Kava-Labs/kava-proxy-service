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
		value        uint64
		expectFound  bool
		expectResult string
	}{
		{1, true, "A"},
		{9, true, "A"},
		{10, true, "B"},
		{15, true, "B"},
		{20, true, "C"},
		{75, true, "C"},
		{100, false, ""},
		{300, false, ""},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Lookup(%d)", tc.value), func(t *testing.T) {
			result, found := intervalmap.Lookup(tc.value)
			require.Equal(t, tc.expectFound, found)
			if tc.expectResult == "" {
				require.Nil(t, result)
			} else {
				require.Equal(t, tc.expectResult, result.String())
			}
		})
	}
}

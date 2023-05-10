package service_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/service"
)

func TestUnitTestGetCacheKey(t *testing.T) {
	tests := []struct {
		name       string
		r          *http.Request
		decodedReq decode.EVMRPCRequestEnvelope
	}{
		{
			name: "basic",
			r: &http.Request{
				URL: &url.URL{
					Path: "/",
				},
			},
			decodedReq: decode.EVMRPCRequestEnvelope{
				JSONRPCVersion: "2.0",
				ID:             1,
				Method:         "eth_getBlockByHash",
				Params:         []interface{}{"0x1234", true},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := service.GetCacheKey(tc.r, &tc.decodedReq)
			if err != nil {
				t.Fatalf("unexpected error: %s", err.Error())
			}
			if key == "" {
				t.Fatal("expected key to be non-empty")
			}
		})
	}
}

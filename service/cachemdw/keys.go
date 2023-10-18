package cachemdw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kava-labs/kava-proxy-service/decode"
)

type CacheItemType int

const (
	CacheItemTypeEVMRequest CacheItemType = iota + 1
)

func (t CacheItemType) String() string {
	switch t {
	case CacheItemTypeEVMRequest:
		return "evm-request"
	default:
		return "unknown"
	}
}

func BuildCacheKey(cachePrefix string, cacheItemType CacheItemType, parts []string) string {
	fullParts := append(
		[]string{
			cachePrefix,
			cacheItemType.String(),
		},
		parts...,
	)

	return strings.Join(fullParts, ":")
}

// GetQueryKey calculates cache key for request
func GetQueryKey(
	cachePrefix string,
	req *decode.EVMRPCRequestEnvelope,
) (string, error) {
	if req == nil {
		return "", fmt.Errorf("request shouldn't be nil")
	}

	// TODO(yevhenii): use stable/sorted JSON serializer
	serializedParams, err := json.Marshal(req.Params)
	if err != nil {
		return "", err
	}

	data := make([]byte, 0)
	data = append(data, []byte(req.Method)...)
	data = append(data, serializedParams...)

	hashedReq := sha256.Sum256(data)
	hashedReqInHex := hex.EncodeToString(hashedReq[:])

	parts := []string{
		req.Method,
		"sha256",
		hashedReqInHex,
	}

	return BuildCacheKey(cachePrefix, CacheItemTypeEVMRequest, parts), nil
}

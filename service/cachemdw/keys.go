package cachemdw

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/kava-labs/kava-proxy-service/decode"
)

type CacheItemType int

const (
	CacheItemTypeQuery CacheItemType = iota + 1
)

func (t CacheItemType) String() string {
	switch t {
	case CacheItemTypeQuery:
		return "query"
	default:
		return "unknown"
	}
}

func BuildCacheKey(cacheItemType CacheItemType, parts []string) string {
	fullParts := append(
		[]string{
			cacheItemType.String(),
		},
		parts...,
	)

	return strings.Join(fullParts, ":")
}

// GetQueryKey calculates cache key for request
func GetQueryKey(
	chainID string,
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

	hashedReq := crypto.Keccak256Hash(data)

	parts := []string{
		chainID,
		req.Method,
		hashedReq.Hex(),
	}

	return BuildCacheKey(CacheItemTypeQuery, parts), nil
}

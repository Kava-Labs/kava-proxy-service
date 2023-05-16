package cachemiddleware

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/kava-labs/kava-proxy-service/decode"
)

// buildCacheKey builds a cache key from a list of parts with the corresponding prefix.
func buildCacheKey(it ItemType, parts []string) string {
	fullParts := append(
		[]string{
			it.Prefix().String(),
		},
		parts...,
	)

	return strings.Join(fullParts, ":")
}

// GetChainKey returns the chain ID cache key for a given request host.
// Mapping: chainkey -> chainID
func GetChainKey(
	host string,
) string {
	parts := []string{
		host,
	}

	return buildCacheKey(ItemTypeChain, parts)
}

// GetQueryKey returns the query cache key for a given request and decoded
// request envelope.
// Mapping: querykey -> request hash
func GetQueryKey(
	chainID string,
	decodedReq *decode.EVMRPCRequestEnvelope,
) (string, error) {
	if decodedReq == nil {
		return "", fmt.Errorf("decoded request is nil")
	}

	reqBytes, err := json.Marshal(decodedReq)
	if err != nil {
		return "", err
	}

	byteHash := crypto.Keccak256Hash(reqBytes)

	parts := []string{
		chainID,
		byteHash.Hex(),
	}

	return buildCacheKey(ItemTypeChain, parts), nil
}

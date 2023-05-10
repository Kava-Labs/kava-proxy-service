package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/kava-labs/kava-proxy-service/decode"
)

func GetCacheKey(r *http.Request, decodedReq *decode.EVMRPCRequestEnvelope) (string, error) {
	reqBytes, err := json.Marshal(decodedReq)
	if err != nil {
		return "", err
	}

	byteHash := crypto.Keccak256Hash(reqBytes)

	parts := []string{
		// TODO: This should be an unique identifier for the chain
		r.URL.Path,
		byteHash.Hex(),
	}

	return strings.Join(parts, ":"), nil
}

package batchmdw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// BatchMiddlewareConfig are the necessary configuration options for the Batch Processing Middleware
type BatchMiddlewareConfig struct {
	ServiceLogger *logging.ServiceLogger

	ContextKeyDecodedRequestBatch  string
	ContextKeyDecodedRequestSingle string
	MaximumBatchSize               int
}

// CreateBatchProcessingMiddleware handles batch EVM requests
// The batched request is pulled from the context.
// Then, each request is proxied via the singleRequestHandler
// and the responses are collated into a single result which is served to the client.
func CreateBatchProcessingMiddleware(
	singleRequestHandler http.HandlerFunc,
	config *BatchMiddlewareConfig,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		batch := r.Context().Value(config.ContextKeyDecodedRequestBatch)
		batchReq, ok := (batch).([]*decode.EVMRPCRequestEnvelope)
		if !ok {
			// this should only happen if the service is misconfigured.
			// the DecodeRequestMiddleware should only route to BatchProcessingMiddleware
			// if it successfully decodes a non-zero length batch of EVM requests.
			config.ServiceLogger.Error().Msg("BatchProcessingMiddleware expected batch EVM request in context but found none")
			// if we can't get decoded request then assign it empty structure to avoid panics
			batchReq = []*decode.EVMRPCRequestEnvelope{}
		}
		if len(batchReq) > config.MaximumBatchSize {
			config.ServiceLogger.Debug().Int("size", len(batchReq)).Int("max allowed", config.MaximumBatchSize).Msg("request batch size too large")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			w.Write([]byte(fmt.Sprintf("request batch size is too large (%d>%d)", len(batchReq), config.MaximumBatchSize)))
			return
		}

		config.ServiceLogger.Trace().Any("batch", batchReq).Msg("[BatchProcessingMiddleware] process EVM batch request")

		reqs := make([]*http.Request, 0, len(batchReq))
		for _, single := range batchReq {
			// proxy service middlewares expect decoded context key to not be set if the request is nil
			// not setting it ensures no nil pointer panics if `null` is included in batch array of requests
			singleRequestContext := r.Context()
			if single != nil {
				singleRequestContext = context.WithValue(r.Context(), config.ContextKeyDecodedRequestSingle, single)
			}

			body, err := json.Marshal(single)
			if err != nil {
				// this shouldn't happen b/c we are marshaling something we unmarshaled.
				config.ServiceLogger.Error().Err(err).Any("request", single).Msg("[BatchProcessingMiddleware] unable to marshal request in batch")
				// if it does happen, degrade gracefully by skipping request.
				continue
			}

			// build request as if it's the only one being requested.
			req, err := http.NewRequestWithContext(singleRequestContext, r.Method, r.URL.String(), bytes.NewBuffer(body))
			if err != nil {
				config.ServiceLogger.Error().Err(err).Any("req", single).Msg("failed to sub-request of batch")
				continue
			}
			req.Host = r.Host
			req.Header = r.Header
			req.Close = true

			reqs = append(reqs, req)
		}

		// process all requests and respond with results in an array
		batchProcessor := NewBatchProcessor(config.ServiceLogger, singleRequestHandler, reqs)
		batchProcessor.RequestAndServe(w)
	}
}

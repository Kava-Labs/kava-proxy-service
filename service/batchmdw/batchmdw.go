package batchmdw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/kava-labs/kava-proxy-service/service/cachemdw"
)

type BatchMiddlewareConfig struct {
	ServiceLogger *logging.ServiceLogger

	ContextKeyDecodedRequestBatch  string
	ContextKeyDecodedRequestSingle string
}

// TODO: replace temp h handler with real deal
func CreateBatchProcessingMiddleware(h http.HandlerFunc, config *BatchMiddlewareConfig) http.HandlerFunc {
	// TODO build or pass in middleware for
	// 1) fetching cached or proxied response for single request
	// 2) caching & metric creation for single requests

	return func(w http.ResponseWriter, r *http.Request) {
		batch := r.Context().Value(config.ContextKeyDecodedRequestBatch)
		batchReq, ok := (batch).([]*decode.EVMRPCRequestEnvelope)
		if !ok {
			// TODO: update this log for batch
			cachemdw.LogCannotCastRequestError(config.ServiceLogger, r)

			// if we can't get decoded request then assign it empty structure to avoid panics
			batchReq = []*decode.EVMRPCRequestEnvelope{}
		}

		config.ServiceLogger.Info().Any("batch", batchReq).Msg("the context's decoded batch!")

		frw := newBatchResponseWriter(w, len(batchReq))

		// TODO: make concurrent!
		// TODO: consider recombining uncached responses before requesting from backend(s)
		for i, single := range batchReq {
			config.ServiceLogger.Debug().Msg(fmt.Sprintf("RELAY REQUEST %d", i+1))
			config.ServiceLogger.Debug().Any("req", single).Str("url", r.URL.String()).Msg("handling individual request from batch")

			rw := frw.next(new(bytes.Buffer))

			// proxy service middlewares expect decoded context key to not be set if the request is nil
			// not setting it ensures no nil pointer panics if `null` is passing in array of requests
			singleRequestContext := r.Context()
			if single != nil {
				singleRequestContext = context.WithValue(r.Context(), config.ContextKeyDecodedRequestSingle, single)
			}

			body, err := json.Marshal(single)
			if err != nil {
				// TODO: this shouldn't happen b/c we are marshaling something we unmarshaled.
				// TODO: report and handle err response
				continue
			}

			req, err := http.NewRequestWithContext(singleRequestContext, r.Method, r.URL.String(), bytes.NewBuffer(body))
			if err != nil {
				panic(fmt.Sprintf("failed build sub-request: %s", err))
			}
			req.Host = r.Host
			req.Header = r.Header

			h.ServeHTTP(rw, req)
		}

		frw.FlushResponses()
		// results := frw.flush()
		// rawMessages := make([]json.RawMessage, 0, len(batchReq))
		// for _, r := range results {
		// 	rawMessages = append(rawMessages, json.RawMessage(r.Bytes()))
		// }

		// // // w.Write("[")

		// // fmt.Printf("%+v\n", rawMessages)

		// res, err := json.Marshal(rawMessages)
		// if err != nil {
		// 	// TODO don't panic!
		// 	panic(fmt.Sprintf("failed to marshal responses: %s\n%+v", err, frw))
		// }

		// // var res bytes.Buffer
		// // res.WriteRune('[')
		// // for i, result := range results {
		// // 	if i != 0 {
		// // 		res.WriteRune(',')
		// // 	}
		// // 	res.Write(result.Bytes())
		// // }
		// // res.WriteRune(']')

		// // TODO: headers!

		// w.Write(res)
	}
}

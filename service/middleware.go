package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/urfave/negroni"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

const (
	DecodedRequestContextKey              = "X-KAVA-PROXY-DECODED-REQUEST-BODY"
	OriginRoundtripLatencyMillisecondsKey = "X-KAVA-PROXY-ORIGIN-ROUNDTRIP-LATENCY-MILLISECONDS"
	RequestStartTimeContextKey            = "X-KAVA-PROXY-REQUEST-START-TIME"
)

// bodySaverResponseWriter implements the interface for http.ResponseWriter
// and stores the status code and header and body for retrieval
// after the response has been read
type bodySaverResponseWriter struct {
	negroni.ResponseWriter
	body *bytes.Buffer
}

// Write writes the response from the origin server to the response
// and copies the response for later use by the proxy service
func (w bodySaverResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)

	if !w.Written() {
		// The status will be StatusOK if WriteHeader has not been called yet
		w.WriteHeader(http.StatusOK)
	}
	size, err := w.ResponseWriter.Write(b)

	return size, err
}

// createRequestLoggingMiddleware returns a handler that logs any request to stdout
// and if able to decode the request to a known type adds it as a context key
// To use the decoded request body, get the value from the context and then
// use type assertion to EVMRPCRequestEnvelope. With this middleware, the request body
// can be read once, and then accessed by all future middleware and the final
// http handler.
func createRequestLoggingMiddleware(h http.HandlerFunc, serviceLogger *logging.ServiceLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStartTimeContext := context.WithValue(r.Context(), RequestStartTimeContextKey, time.Now())

		var rawBody []byte

		if r.Body != nil {
			var rawBodyBuffer bytes.Buffer

			// Read the request body
			body := io.TeeReader(r.Body, &rawBodyBuffer)

			var err error

			rawBody, err = ioutil.ReadAll(body)

			if err != nil {
				serviceLogger.Debug().Msg(fmt.Sprintf("error %s reading request body %s", err, body))

				h.ServeHTTP(w, r)

				return
			}

			// Repopulate the request body for the ultimate consumer of this request
			r.Body = ioutil.NopCloser(&rawBodyBuffer)
		}

		decodedRequest, err := decode.DecodeEVMRPCRequest(rawBody)

		if err != nil {
			serviceLogger.Debug().Msg(fmt.Sprintf("error %s parsing of request body %s", err, rawBody))

			h.ServeHTTP(w, r)

			return
		}

		serviceLogger.Debug().Msg(fmt.Sprintf("decoded request body %+v", decodedRequest))

		decodedRequestBodyContext := context.WithValue(requestStartTimeContext, DecodedRequestContextKey, decodedRequest)

		h.ServeHTTP(w, r.WithContext(decodedRequestBodyContext))
	}
}

// create the main service middleware for introspecting and transforming
// the request and the backend origin server(s) response(s)
func createProxyRequestMiddleware(next http.Handler, config config.Config, serviceLogger *logging.ServiceLogger) http.HandlerFunc {
	// create an http handler that will proxy any request to the specified URL
	proxy := httputil.NewSingleHostReverseProxy(&config.ProxyBackendHostURLParsed)

	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			serviceLogger.Debug().Msg(fmt.Sprintf("proxying request %+v", r))

			proxyRequestAt := time.Now()

			// set up response writer for copying the response from the backend server
			// for use out of band of the request-response cycle
			lrw := &bodySaverResponseWriter{ResponseWriter: negroni.NewResponseWriter(w), body: bytes.NewBufferString("")}

			// proxy the request to the backend origin server
			p.ServeHTTP(lrw, r)

			serviceLogger.Debug().Msg(fmt.Sprintf("response %+v \nheaders %+v \nstatus %+v for request %+v", lrw.Status(), lrw.Header(), lrw.body, r))

			// calculate how long it took to proxy the request
			requestRoundtrip := time.Since(proxyRequestAt)

			serviceLogger.Debug().Msg(fmt.Sprintf("proxy request latency %v for %+v", requestRoundtrip, r))

			originRoundtripLatencyContext := context.WithValue(r.Context(), OriginRoundtripLatencyMillisecondsKey, requestRoundtrip.Milliseconds())

			next.ServeHTTP(lrw, r.WithContext(originRoundtripLatencyContext))
		}
	}

	return handler(proxy)
}

func createAfterProxyFinalizer(service *ProxyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// parse values added to the context by handlers further up the middleware chain
		rawDecodedRequestBody := r.Context().Value(DecodedRequestContextKey)
		decodedRequestBody, ok := (rawDecodedRequestBody).(*decode.EVMRPCRequestEnvelope)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawDecodedRequestBody, DecodedRequestContextKey))

			return
		}

		rawOriginRoundtripLatencyMilliseconds := r.Context().Value(OriginRoundtripLatencyMillisecondsKey)
		originRoundtripLatencyMilliseconds, ok := rawOriginRoundtripLatencyMilliseconds.(int64)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawOriginRoundtripLatencyMilliseconds, OriginRoundtripLatencyMillisecondsKey))

			return
		}

		rawRequestStartTime := r.Context().Value(RequestStartTimeContextKey)
		requestStartTime, ok := rawRequestStartTime.(time.Time)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawRequestStartTime, RequestStartTimeContextKey))

			return
		}

		// create a metric for the request
		metric := database.ProxiedRequestMetric{
			MethodName:                  decodedRequestBody.Method,
			ResponseLatencyMilliseconds: originRoundtripLatencyMilliseconds,
			RequestTime:                 requestStartTime,
		}

		// save metric to database
		err := metric.Save(r.Context(), service.database.DB)

		if err != nil {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("error %s saving metric %+v using database %+v", err, metric, service.database))
			return
		}

		service.ServiceLogger.Debug().Msg(fmt.Sprintf("created request metric"))
	}
}

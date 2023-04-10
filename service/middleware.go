package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/urfave/negroni"

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/decode"
	"github.com/kava-labs/kava-proxy-service/logging"
)

const (
	DefaultAnonymousUserAgent = "anon"
	// Service defined context keys
	DecodedRequestContextKey              = "X-KAVA-PROXY-DECODED-REQUEST-BODY"
	OriginRoundtripLatencyMillisecondsKey = "X-KAVA-PROXY-ORIGIN-ROUNDTRIP-LATENCY-MILLISECONDS"
	RequestStartTimeContextKey            = "X-KAVA-PROXY-REQUEST-START-TIME"
	RequestHostnameContextKey             = "X-KAVA-PROXY-REQUEST-HOSTNAME"
	RequestIPContextKey                   = "X-KAVA-PROXY-REQUEST-IP"
	RequestUserAgentContextKey            = "X-KAVA-PROXY-USER-AGENT"
	RequestRefererContextKey              = "X-KAVA-PROXY-REFERER"
	RequestOriginContextKey               = "X-KAVA-PROXY-ORIGIN"
	// Values defined by upstream services
	LoadBalancerForwardedForHeaderKey = "X-Forwarded-For"
	UserAgentHeaderkey                = "User-Agent"
	RefererHeaderKey                  = "Referer"
	OriginHeaderKey                   = "Origin"
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

			rawBody, err = io.ReadAll(body)

			if err != nil {
				serviceLogger.Debug().Msg(fmt.Sprintf("error %s reading request body %s", err, body))

				h.ServeHTTP(w, r)

				return
			}

			// Repopulate the request body for the ultimate consumer of this request
			r.Body = io.NopCloser(&rawBodyBuffer)
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
	reverseProxyForHost := make(map[string]*httputil.ReverseProxy)

	for host, proxyBackendURL := range config.ProxyBackendHostURLMapParsed {
		serviceLogger.Debug().Msg(fmt.Sprintf("creating reverse proxy for host %s to %+v", host, proxyBackendURL))

		targetURL := config.ProxyBackendHostURLMapParsed[host]

		reverseProxyForHost[host] = httputil.NewSingleHostReverseProxy(&targetURL)
	}

	handler := func(proxies map[string]*httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			serviceLogger.Debug().Msg(fmt.Sprintf("proxying request %+v", r))

			proxyRequestAt := time.Now()

			// set up response writer for copying the response from the backend server
			// for use out of band of the request-response cycle
			lrw := &bodySaverResponseWriter{ResponseWriter: negroni.NewResponseWriter(w), body: bytes.NewBufferString("")}

			// proxy the request to the backend origin server
			// based on the request host
			proxy, ok := proxies[r.Host]

			if !ok {
				serviceLogger.Error().Msg(fmt.Sprintf("no matching proxy for host %s for request %+v\n configured proxies %+v", r.Host, r, proxies))

				w.WriteHeader(http.StatusBadGateway)

				w.Write([]byte("no proxy backend configured for request host"))

				return
			}

			proxy.ServeHTTP(lrw, r)

			serviceLogger.Debug().Msg(fmt.Sprintf("response %+v \nheaders %+v \nstatus %+v for request %+v", lrw.Status(), lrw.Header(), lrw.body, r))

			// calculate how long it took to proxy the request
			requestRoundtrip := time.Since(proxyRequestAt)

			serviceLogger.Debug().Msg(fmt.Sprintf("proxy request latency %v for %+v", requestRoundtrip, r))

			originRoundtripLatencyContext := context.WithValue(r.Context(), OriginRoundtripLatencyMillisecondsKey, requestRoundtrip.Milliseconds())

			// extract the original hostname the request was sent to
			requestHostnameContext := context.WithValue(originRoundtripLatencyContext, RequestHostnameContextKey, r.Host)

			enrichedContext := requestHostnameContext

			// parse the remote address of the request for use below
			remoteAddressParts := strings.Split(r.RemoteAddr, ":")

			// extract the ip of the client that made the request
			requestIPHeaderValues := r.Header[LoadBalancerForwardedForHeaderKey]

			// when deployed in a production environment there will often
			// be a load balancer in front of the proxy service that first handles
			// the request, in which case the ip of the original request should be tracked
			// otherwise the ip of the connecting client
			if len(requestIPHeaderValues) == 1 {
				enrichedContext = context.WithValue(enrichedContext, RequestIPContextKey, requestIPHeaderValues[0])

			} else {
				enrichedContext = context.WithValue(enrichedContext, RequestIPContextKey, remoteAddressParts[0])
			}

			// similarly `RefererHeaderKey`  will be set by the load balancer
			// and should be used if present otherwise default to the requester's ip
			requestRefererHeaderValues := r.Header[RefererHeaderKey]

			if len(requestRefererHeaderValues) == 1 {
				enrichedContext = context.WithValue(enrichedContext, RequestRefererContextKey, requestRefererHeaderValues[0])
			} else {
				enrichedContext = context.WithValue(enrichedContext, RequestRefererContextKey, fmt.Sprintf("http://%s", remoteAddressParts[0]))
			}

			// `OriginHeaderKey` may be set by the load balancer
			// and should be used if present otherwise default to the requester's ip
			requestOriginHeaderValues := r.Header[OriginHeaderKey]

			if len(requestOriginHeaderValues) == 1 {
				enrichedContext = context.WithValue(enrichedContext, RequestOriginContextKey, requestOriginHeaderValues[0])
			} else {
				enrichedContext = context.WithValue(enrichedContext, RequestOriginContextKey, fmt.Sprintf("http://%s", remoteAddressParts[0]))
			}

			// extract the user agent of the requestor
			userAgentHeaderValues := r.Header[UserAgentHeaderkey]

			// add user agent to context if present
			// otherwise defaulting to `DefaultAnonymousUserAgent`
			if len(userAgentHeaderValues) == 1 {
				enrichedContext = context.WithValue(enrichedContext, RequestUserAgentContextKey, userAgentHeaderValues[0])
			} else {
				enrichedContext = context.WithValue(enrichedContext, RequestUserAgentContextKey, DefaultAnonymousUserAgent)
			}

			next.ServeHTTP(lrw, r.WithContext(enrichedContext))
		}
	}

	return handler(reverseProxyForHost)
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

		rawRequestHostname := r.Context().Value(RequestHostnameContextKey)
		requestHostname, ok := rawRequestHostname.(string)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawRequestHostname, RequestHostnameContextKey))

			return
		}

		rawRequestIP := r.Context().Value(RequestIPContextKey)
		requestIP, ok := rawRequestIP.(string)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawRequestIP, RequestIPContextKey))

			return
		}

		rawUserAgent := r.Context().Value(RequestUserAgentContextKey)
		userAgent, ok := rawUserAgent.(string)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawUserAgent, RequestUserAgentContextKey))

			return
		}

		rawReferer := r.Context().Value(RequestRefererContextKey)
		referer, ok := rawReferer.(string)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawReferer, RequestRefererContextKey))

			return
		}

		rawOrigin := r.Context().Value(RequestRefererContextKey)
		origin, ok := rawOrigin.(string)

		if !ok {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawOrigin, RequestOriginContextKey))

			return
		}

		// create a metric for the request
		metric := database.ProxiedRequestMetric{
			MethodName:                  decodedRequestBody.Method,
			ResponseLatencyMilliseconds: originRoundtripLatencyMilliseconds,
			RequestTime:                 requestStartTime,
			Hostname:                    requestHostname,
			RequestIP:                   requestIP,
			UserAgent:                   &userAgent,
			Referer:                     &referer,
			Origin:                      &origin,
		}

		// save metric to database
		err := metric.Save(r.Context(), service.Database.DB)

		if err != nil {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("error %s saving metric %+v using database %+v", err, metric, service.Database))
			return
		}

		service.ServiceLogger.Debug().Msg(fmt.Sprintf("created request metric"))
	}
}
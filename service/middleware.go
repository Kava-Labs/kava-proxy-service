package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/urfave/negroni"

	"github.com/kava-labs/kava-proxy-service/cache"
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
	CachedContextKey                      = "X-KAVA-PROXY-CACHED"

	// Values defined by upstream services
	LoadBalancerForwardedForHeaderKey = "X-Forwarded-For"
	UserAgentHeaderkey                = "User-Agent"
	RefererHeaderKey                  = "Referer"
	OriginHeaderKey                   = "Origin"
	CacheHitHeaderKey                 = "X-Cache"

	// Values defined by this service
	CacheHitHeaderValue  = "HIT"
	CacheMissHeaderValue = "MISS"
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

		serviceLogger.Trace().Msg(fmt.Sprintf("decoded request body %+v", decodedRequest))

		decodedRequestBodyContext := context.WithValue(requestStartTimeContext, DecodedRequestContextKey, decodedRequest)

		h.ServeHTTP(w, r.WithContext(decodedRequestBodyContext))
	}
}

func createCacheRequestMiddleware(
	next http.Handler,
	config config.Config,
	service *ProxyService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawDecodedRequestBody := r.Context().Value(DecodedRequestContextKey)
		decodedReq, ok := (rawDecodedRequestBody).(*decode.EVMRPCRequestEnvelope)
		uncachedContext := context.WithValue(r.Context(), CachedContextKey, false)

		// Skip caching if no decoded request body
		if !ok {
			service.Logger.Debug().Msg(fmt.Sprintf("error asserting decoded request body %s", rawDecodedRequestBody))

			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// TODO: We may want to have a different set of cacheable methods
		// that is a smaller list than ExtractBlockNumberFromEVMRPCRequest uses.
		// Skip caching if we can't extract block number
		blockNumber, err := decodedReq.ExtractBlockNumberFromEVMRPCRequest(r.Context(), service.evmClient)
		if err != nil || blockNumber <= 0 {
			service.Logger.Debug().Msg(fmt.Sprintf("error extracting block number from request %s", rawDecodedRequestBody))

			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// Build the cache key
		key, err := cache.GetQueryKey(r, decodedReq)
		if err != nil {
			service.Logger.Debug().Msg(fmt.Sprintf("error determining cache key %s", rawDecodedRequestBody))

			// Skip caching if we can't decode the request body
			next.ServeHTTP(w, r.WithContext(uncachedContext))
			return
		}

		// Check if the request is cached
		cached, err := service.Cache.Get(context.Background(), key)
		if err != nil {
			// Not cached, only include the cache miss header if we were able
			// to actually check the cache.
			service.Logger.Debug().Msg(fmt.Sprintf("request cache miss: %s", err))
			w.Header().Add(CacheHitHeaderKey, CacheMissHeaderValue)

			// Serve the request and cache the response
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r.WithContext(uncachedContext))
			result := rec.Result()

			// Check if the response is successful
			if result.StatusCode != http.StatusOK {
				return
			}

			// Copy the response back to the original response
			body := rec.Body.Bytes()
			for k, v := range result.Header {
				w.Header().Set(k, strings.Join(v, ","))
			}
			w.WriteHeader(result.StatusCode)
			w.Write(body)

			// TODO: Determine if the response should be cached or not.
			// Requests for future blocks should not be cached.

			// Cache the response bytes after writing response
			service.Logger.Debug().Msg(fmt.Sprintf("caching response for key %s", key))
			if err := service.Cache.Set(
				context.Background(),
				key,
				body,
				config.CacheTTL,
			); err != nil {
				service.Logger.Debug().Msg(fmt.Sprintf("error caching response %s", err))
			}

			return
		}

		// Serve the cached response
		service.Logger.Debug().Msg(fmt.Sprintf("found cached response for key %s", key))
		w.Header().Add(CacheHitHeaderKey, CacheHitHeaderValue)
		w.Header().Add("Content-Type", http.DetectContentType(cached))
		w.Write(cached)

		// Make sure the next handler knows the response was served from cache
		// and to not hit the origin server. This does not use the uncachedContext
		cachedContext := context.WithValue(r.Context(), CachedContextKey, true)
		next.ServeHTTP(w, r.WithContext(cachedContext))
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
			serviceLogger.Trace().Msg(fmt.Sprintf("proxying request %+v", r))

			proxyRequestAt := time.Now()

			// set up response writer for copying the response from the backend server
			// for use out of band of the request-response cycle
			lrw := &bodySaverResponseWriter{
				ResponseWriter: negroni.NewResponseWriter(w),
				body:           bytes.NewBufferString(""),
			}

			// proxy the request to the backend origin server
			// based on the request host
			proxy, ok := proxies[r.Host]

			if !ok {
				serviceLogger.Error().Msg(fmt.Sprintf("no matching proxy for host %s for request %+v\n configured proxies %+v", r.Host, r, proxies))

				w.WriteHeader(http.StatusBadGateway)

				w.Write([]byte("no proxy backend configured for request host"))

				return
			}

			// ensure the last set value of LoadBalancerForwardedForHeaderKey wins
			// to prevent clients from forging the value of the header in an attempt
			// to bypass an ip based rate limit
			requestIPHeaderValues := r.Header[LoadBalancerForwardedForHeaderKey]

			if len(requestIPHeaderValues) > 1 {
				serviceLogger.Trace().Msg(fmt.Sprintf("found more than value for %s header: %s, clearing all but the last", LoadBalancerForwardedForHeaderKey, requestIPHeaderValues))
				r.Header.Set(LoadBalancerForwardedForHeaderKey, requestIPHeaderValues[len(requestIPHeaderValues)-1])
			}

			// Only proxy the request if it's not cached
			isCachedVal := r.Context().Value(CachedContextKey)
			isCached := isCachedVal != nil && isCachedVal.(bool)
			if !isCached {
				proxy.ServeHTTP(lrw, r)
			}

			serviceLogger.Trace().Msg(fmt.Sprintf("response %+v \nheaders %+v \nstatus %+v for request %+v", lrw.Status(), lrw.Header(), lrw.body, r))

			// calculate how long it took to proxy the request
			requestRoundtrip := time.Since(proxyRequestAt)

			serviceLogger.Trace().Msg(fmt.Sprintf("proxy request latency %v for %+v", requestRoundtrip, r))

			originRoundtripLatencyContext := context.WithValue(r.Context(), OriginRoundtripLatencyMillisecondsKey, requestRoundtrip.Milliseconds())

			// extract the original hostname the request was sent to
			requestHostnameContext := context.WithValue(originRoundtripLatencyContext, RequestHostnameContextKey, r.Host)

			enrichedContext := requestHostnameContext

			// parse the remote address of the request for use below
			remoteAddressParts := strings.Split(r.RemoteAddr, ":")

			// extract the ip of the client that made the request
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

// createAfterProxyFinalizer returns a middleware function that expects
// to run after a request has been proxied and attempts to create a metric
// for the request by parsing values in the context set by handlers
// further up the middleware chain
func createAfterProxyFinalizer(service *ProxyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// parse values added to the context by handlers further up the middleware chain
		rawDecodedRequestBody := r.Context().Value(DecodedRequestContextKey)
		decodedRequestBody, ok := (rawDecodedRequestBody).(*decode.EVMRPCRequestEnvelope)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawDecodedRequestBody, DecodedRequestContextKey))

			return
		}

		rawOriginRoundtripLatencyMilliseconds := r.Context().Value(OriginRoundtripLatencyMillisecondsKey)
		originRoundtripLatencyMilliseconds, ok := rawOriginRoundtripLatencyMilliseconds.(int64)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawOriginRoundtripLatencyMilliseconds, OriginRoundtripLatencyMillisecondsKey))

			return
		}

		rawRequestStartTime := r.Context().Value(RequestStartTimeContextKey)
		requestStartTime, ok := rawRequestStartTime.(time.Time)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawRequestStartTime, RequestStartTimeContextKey))

			return
		}

		rawRequestHostname := r.Context().Value(RequestHostnameContextKey)
		requestHostname, ok := rawRequestHostname.(string)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawRequestHostname, RequestHostnameContextKey))

			return
		}

		rawRequestIP := r.Context().Value(RequestIPContextKey)
		requestIP, ok := rawRequestIP.(string)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawRequestIP, RequestIPContextKey))

			return
		}

		rawUserAgent := r.Context().Value(RequestUserAgentContextKey)
		userAgent, ok := rawUserAgent.(string)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawUserAgent, RequestUserAgentContextKey))

			return
		}

		rawReferer := r.Context().Value(RequestRefererContextKey)
		referer, ok := rawReferer.(string)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawReferer, RequestRefererContextKey))

			return
		}

		rawOrigin := r.Context().Value(RequestRefererContextKey)
		origin, ok := rawOrigin.(string)

		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawOrigin, RequestOriginContextKey))

			return
		}

		var blockNumber *int64
		rawBlockNumber, err := decodedRequestBody.ExtractBlockNumberFromEVMRPCRequest(r.Context(), service.evmClient)

		if err != nil {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("error %s parsing block number from request %+v", err, decodedRequestBody))

			blockNumber = nil
		} else {
			blockNumber = &rawBlockNumber
		}

		rawIsCached := r.Context().Value(CachedContextKey)
		isCachedVal, ok := rawIsCached.(bool)
		if !ok {
			service.ServiceLogger.Debug().Msg(fmt.Sprintf("invalid context value %+v for value %s", rawIsCached, CachedContextKey))

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
			BlockNumber:                 blockNumber,
			CacheHit:                    isCachedVal,
		}

		// save metric to database
		err = metric.Save(context.Background(), service.Database.DB)

		if err != nil {
			service.ServiceLogger.Error().Msg(fmt.Sprintf("error %s saving metric %+v using database %+v", err, metric, service.Database))
			return
		}

		service.ServiceLogger.Trace().Msg("created request metric")
	}
}

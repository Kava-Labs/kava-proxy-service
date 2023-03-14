# Service Middleware

The proxy service uses a composable stack of middleware functions where each middleware is responsible for a focused sub-set of those features.

## Abstract Middleware

Middleware take the form of a function that accepts as params the `next` middleware function to call, and other parameters such as a logger or database client for debugging and storing request and response values.

```golang
func noOpMiddleware(next http.HandlerFunc, otherParams ...interface{}) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        next.ServeHTTP(w, r)
    }
}
```

Any modifications that the middleware function makes to the request or response before calling `next.ServeHTTP()` will be observable by the `next` middleware function, any modifications made after `next.ServeHTTP()` has been called will not.

## Execution order

The earlier the middleware is instantiated, the later it will run. For example the first middleware created by the proxy service is the middleware that will run after the request has been logged and proxied, thereby allowing it to access both the recorded request body and response body, and any context enrichment added by prior middleware.

```golang
service := ProxyService{}

// create an http router for registering handlers for a given route
mux := http.NewServeMux()

// will run after the proxy middleware handler and is
// the final function called after all other middleware
// allowing it to access values added to the request context
// to do things like metric the response and cache the response
afterProxyFinalizer := createAfterProxyFinalizer(&service)

// create an http handler that will proxy any request to the specified URL
proxyMiddleware := createProxyRequestMiddleware(afterProxyFinalizer, config, serviceLogger)

// create an http handler that will log the request to stdout
// this handler will run before the proxyMiddleware handler
requestLoggingMiddleware := createRequestLoggingMiddleware(proxyMiddleware, serviceLogger)

// register middleware chain as the default handler for any request to the proxy service
mux.HandleFunc("/", requestLoggingMiddleware)

// create an http server for the caller to start on demand with a call to ProxyService.Run()
server := &http.Server{
    Addr:    fmt.Sprintf(":%s", config.ProxyServicePort),
    Handler: mux,
}
```

## Middleware

### Request Logging Middleware

1. Logs the request body to stdout and stores a parsed version of the request body in the context key `X-KAVA-PROXY-DECODED-REQUEST-BODY` for use by other middleware.

### Proxy Middleware

1. Proxies the request to the configured backend origin server.

1. Times the roundtrip latency for the response from the backend origin server and stores the latency in the context key `X-KAVA-PROXY-ORIGIN-ROUNDTRIP-LATENCY-MILLISECONDS` for use by other middleware.

### After Proxy Middleware

1. Parses the request body and latency from context key values and creates a request metric for the proxied request.

// package service provides functions and methods
// for creating and running the api of the proxy service
package service

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// ProxyService represents an instance of the proxy service API
type ProxyService struct {
	httpProxy *http.Server
	*logging.ServiceLogger
}

// New returns a new ProxyService with the specified config and error (if any)
func New(config config.Config, serviceLogger *logging.ServiceLogger) (ProxyService, error) {
	// create an http router for registering handlers for a given route
	mux := http.NewServeMux()

	// create an http handler that will proxy any request to the specified URL
	proxy := httputil.NewSingleHostReverseProxy(&config.ProxyBackendHostURLParsed)

	// create the main service handler for introspecting and transforming
	// the request and the backend origin server(s) response(s)
	// TODO: break out into more composable middleware
	handler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			serviceLogger.Debug().Msg(fmt.Sprintf("proxying request %+v", r))
			// TODO: Parse request, store request metric in database
			// TODO: Set Proxy headers
			// TODO: Time response latency
			p.ServeHTTP(w, r)
		}
	}

	// register proxy handler as the default handler for any request
	mux.HandleFunc("/", handler(proxy))

	// create an http server for the caller to start at their own discretion
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.ProxyServicePort),
		Handler: mux,
	}

	return ProxyService{
		httpProxy:     server,
		ServiceLogger: serviceLogger,
	}, nil
}

// Run runs the proxy service, returning error (if any) in the event
// the proxy service stops
func (p *ProxyService) Run() error {
	return p.httpProxy.ListenAndServe()
}

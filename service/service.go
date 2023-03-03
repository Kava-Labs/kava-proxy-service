// package service provides functions and methods
// for creating and running the api of the proxy service
package service

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
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

			var rawBody []byte
			if r.Body != nil {
				var rawBodyBuffer bytes.Buffer
				// Read the request body
				body := io.TeeReader(r.Body, &rawBodyBuffer)
				var err error
				rawBody, err = ioutil.ReadAll(body)
				if err != nil {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}
				// Repopulate the request body for the ultimate consumer of this request
				r.Body = ioutil.NopCloser(&rawBodyBuffer)
			}

			serviceLogger.Debug().Msg(fmt.Sprintf("request body %s", rawBody))
			// TODO: Set Proxy headers
			// TODO: Start timing response latency
			p.ServeHTTP(w, r)
			// TODO: get response code
			// TODO: calculate response latency
			// TODO: store request metric in database
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

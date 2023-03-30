package service

import (
	"errors"
	"fmt"
	"net/http"
)

// createHealthcheckHandler creates a health check handler function that
// will respond 200 ok if the proxy service is able to connect to
// it's dependencies and functioning as expected
func createHealthcheckHandler(service *ProxyService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var combinedErrors error

		service.Debug().Msg("/healthcheck called")

		// check that the database is reachable
		err := service.Database.HealthCheck()

		if err != nil {
			errMsg := fmt.Errorf("proxy service unable to connect to database")
			combinedErrors = errors.Join(combinedErrors, errMsg)
		}

		if combinedErrors != nil {
			w.WriteHeader(http.StatusInternalServerError)

			w.Write([]byte(combinedErrors.Error()))

			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("proxy service is healthy"))
	}
}

// createServicecheckHandler creates a service check handler function that
// will respond 200 ok if the proxy service is running
func createServicecheckHandler(service *ProxyService) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		service.Debug().Msg("/servicecheck called")

		w.WriteHeader(http.StatusOK)

		w.Write([]byte("proxy service is in service"))
	}
}

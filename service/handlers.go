package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/kava-labs/kava-proxy-service/clients/database"
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

		if service.Cache.IsCacheEnabled() {
			// check that the cache is reachable
			err := service.Cache.Healthcheck(context.Background())
			if err != nil {
				service.Logger.Error().
					Err(err).
					Msg("cache healthcheck failed")

				errMsg := fmt.Errorf("proxy service unable to connect to cache: %v", err)
				combinedErrors = errors.Join(combinedErrors, errMsg)
			}
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

// createDatabaseStatusHandler creates a database status handler
// function responding to requests for the status of database related
// operations such as proxied request metrics compaction and
// partitioning
func createDatabaseStatusHandler(service *ProxyService, db *database.PostgresClient) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		service.Debug().Msg("/database/status called")

		proxiedRequestMetricPartitionsCount, err := database.CountAttachedProxiedRequestMetricPartitions(r.Context(), db.DB)

		if err != nil {
			service.Error().Msg(fmt.Sprintf("error %s getting proxiedRequestMetricPartitionsCount", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		proxiedRequestMetricLatestAttachedPartitionName, err := database.GetLastCreatedAttachedProxiedRequestMetricsPartitionName(r.Context(), db.DB)

		if err != nil {
			service.Error().Msg(fmt.Sprintf("error %s getting proxiedRequestMetricPartitionsCount", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// prepare response for client
		w.WriteHeader(http.StatusOK)

		response := DatabaseStatusResponse{
			TotalProxiedRequestMetricPartitions:          proxiedRequestMetricPartitionsCount,
			LatestProxiedRequestMetricPartitionTableName: proxiedRequestMetricLatestAttachedPartitionName,
		}

		// return response for client
		if err := MarshalJSONResponse(&response, w); err != nil {
			service.Error().Msg(fmt.Sprintf("error %s encoding %+v to json", err, response))
		}
	}
}

// MarshalJSONResponse marshals an interface into the response body and sets JSON content type headers
func MarshalJSONResponse(obj interface{}, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(obj); err != nil {
		return err
	}
	return nil
}

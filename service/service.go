// package service provides functions and methods
// for creating and running the api of the proxy service
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

	"github.com/kava-labs/kava-proxy-service/clients/database"
	"github.com/kava-labs/kava-proxy-service/clients/database/migrations"
	"github.com/kava-labs/kava-proxy-service/config"
	"github.com/kava-labs/kava-proxy-service/logging"
)

// ProxyService represents an instance of the proxy service API
type ProxyService struct {
	database  *database.PostgresClient
	httpProxy *http.Server
	*logging.ServiceLogger
}

// New returns a new ProxyService with the specified config and error (if any)
func New(ctx context.Context, config config.Config, serviceLogger *logging.ServiceLogger) (ProxyService, error) {
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

	// create database client
	db, err := createDatabase(ctx, config, serviceLogger)

	if err != nil {
		return ProxyService{}, err
	}

	return ProxyService{
		httpProxy:     server,
		ServiceLogger: serviceLogger,
		database:      db,
	}, nil
}

func createDatabase(ctx context.Context, config config.Config, logger *logging.ServiceLogger) (*database.PostgresClient, error) {
	databaseConfig := database.PostgresDatabaseConfig{
		DatabaseName:          config.DatabaseName,
		DatabaseEndpointURL:   config.DatabaseEndpointURL,
		DatabaseUserName:      config.DatabaseUserName,
		DatabasePassword:      config.DatabasePassword,
		SSLEnabled:            config.DatabaseSSLEnabled,
		QueryLoggingEnabled:   config.DatabaseQueryLoggingEnabled,
		Logger:                logger,
		RunDatabaseMigrations: config.RunDatabaseMigrations,
	}

	serviceDatabase, err := database.NewPostgresClient(databaseConfig)

	if err != nil {
		logger.Error().Msg(fmt.Sprintf("error %s creating database using config %+v", err, databaseConfig))

		return &database.PostgresClient{}, err
	}

	if !databaseConfig.RunDatabaseMigrations {
		logger.Debug().Msg("skipping attempting to run migrations on database since RUN_DATABASE_MIGRATIONS was false")
		return &serviceDatabase, nil
	}

	// wait for database to be reachable
	var databaseOnline bool
	for !databaseOnline {
		err = serviceDatabase.HealthCheck()

		if err != nil {
			logger.Debug().Msg("unable to connect to database, will retry in 1 second")

			time.Sleep(1 * time.Second)

			continue
		}

		logger.Debug().Msg("connected to database")

		databaseOnline = true
	}

	logger.Debug().Msg("running migrations on database")

	migrations, err := database.Migrate(ctx, serviceDatabase.DBConnection, *migrations.Migrations, logger)

	if err != nil {
		logger.Error().Msg(fmt.Sprintf("error %s running migrations on database, will retry in 1 second", err))

	}

	logger.Debug().Msg(fmt.Sprintf("run migrations %+v \n last group %+v \n unapplied %+v", migrations.Applied(), migrations.LastGroup(), migrations.Unapplied()))

	return &serviceDatabase, err
}

// Run runs the proxy service, returning error (if any) in the event
// the proxy service stops
func (p *ProxyService) Run() error {
	return p.httpProxy.ListenAndServe()
}

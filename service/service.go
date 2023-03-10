// package service provides functions and methods
// for creating and running the api of the proxy service
package service

import (
	"context"
	"fmt"
	"net/http"
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

	// create database client
	db, err := createDatabaseClient(ctx, config, serviceLogger)

	if err != nil {
		return ProxyService{}, err
	}

	service = ProxyService{
		httpProxy:     server,
		ServiceLogger: serviceLogger,
		database:      db,
	}

	return service, nil
}

// createDatabaseClient creates a connection to the database
// using the specified config and runs migrations
// (only if migration flag in config is true) returning the
// returning the database connection and error (if any)
func createDatabaseClient(ctx context.Context, config config.Config, logger *logging.ServiceLogger) (*database.PostgresClient, error) {
	databaseConfig := database.PostgresDatabaseConfig{
		DatabaseName:          config.DatabaseName,
		DatabaseEndpointURL:   config.DatabaseEndpointURL,
		DatabaseUsername:      config.DatabaseUserName,
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

	migrations, err := database.Migrate(ctx, serviceDatabase.DB, *migrations.Migrations, logger)

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

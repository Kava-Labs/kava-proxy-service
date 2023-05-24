package database

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"time"

	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

// PostgresDatabaseConfig contains values for creating a
// new connection to a postgres database
type PostgresDatabaseConfig struct {
	DatabaseName                     string
	DatabaseEndpointURL              string
	DatabaseUsername                 string
	DatabasePassword                 string
	ReadTimeoutSeconds               int64
	DatabaseMaxIdleConnections       int64
	DatabaseConnectionMaxIdleSeconds int64
	DatabaseMaxOpenConnections       int64
	SSLEnabled                       bool
	QueryLoggingEnabled              bool
	RunDatabaseMigrations            bool
	Logger                           *logging.ServiceLogger
}

// PostgresClient wraps a connection to a postgres database
type PostgresClient struct {
	*bun.DB
}

// NewPostgresClient returns a new connection to the specified
// postgres data and error (if any)
func NewPostgresClient(config PostgresDatabaseConfig) (PostgresClient, error) {
	// configure postgres database connection options
	var pgOptions *pgdriver.Connector

	// TODO: figure out if the library supports controlling
	// TLS outside of the "WithInsecure" method which can't
	// be undone or applied after connector creation
	if config.SSLEnabled {
		pgOptions =
			pgdriver.NewConnector(
				pgdriver.WithAddr(config.DatabaseEndpointURL),
				pgdriver.WithUser(config.DatabaseUsername),
				pgdriver.WithTLSConfig(&tls.Config{InsecureSkipVerify: false}),
				pgdriver.WithPassword(config.DatabasePassword),
				pgdriver.WithDatabase(config.DatabaseName),
				pgdriver.WithReadTimeout(time.Second*time.Duration(config.ReadTimeoutSeconds)),
			)
	} else {
		pgOptions = pgdriver.NewConnector(
			pgdriver.WithAddr(config.DatabaseEndpointURL),
			pgdriver.WithUser(config.DatabaseUsername),
			pgdriver.WithInsecure(true),
			pgdriver.WithPassword(config.DatabasePassword),
			pgdriver.WithDatabase(config.DatabaseName),
			pgdriver.WithReadTimeout(time.Second*time.Duration(config.ReadTimeoutSeconds)),
		)
	}

	config.Logger.Debug().Msg(fmt.Sprintf("creating database client with options %+v", pgOptions.Config()))

	// connect to the database
	sqldb := sql.OpenDB(pgOptions)

	// configure connection limits
	// https://go.dev/doc/database/manage-connections#connection_pool_properties
	sqldb.SetMaxIdleConns(int(config.DatabaseMaxIdleConnections))
	sqldb.SetConnMaxIdleTime(time.Second * time.Duration(config.DatabaseConnectionMaxIdleSeconds))
	sqldb.SetMaxOpenConns(int(config.DatabaseMaxOpenConnections))

	db := bun.NewDB(sqldb, pgdialect.New())

	// set up logging on database if requested
	if config.QueryLoggingEnabled {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	return PostgresClient{
		DB: db,
	}, nil
}

// HealthCheck returns an error if the database can not
// be connected to and queried, nil otherwise
func (pg *PostgresClient) HealthCheck() error {
	_, err := pg.Query(`SELECT 1;`)
	return err
}

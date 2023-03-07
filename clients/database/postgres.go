package database

import (
	"crypto/tls"
	"database/sql"
	"fmt"

	"github.com/kava-labs/kava-proxy-service/logging"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

// PostgresDatabaseConfig contains values for creating a
// new connection to a postgres database
type PostgresDatabaseConfig struct {
	DatabaseName        string
	DatabaseEndpointURL string
	DatabaseUserName    string
	DatabasePassword    string
	SSLEnabled          bool
	QueryLoggingEnabled bool
	Logger              *logging.ServiceLogger
}

// PostgresClient wraps a connection to a postgres database
type PostgresClient struct {
	DBConnection *bun.DB
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
				pgdriver.WithUser(config.DatabaseUserName),
				pgdriver.WithTLSConfig(&tls.Config{InsecureSkipVerify: false}),
				pgdriver.WithPassword(config.DatabasePassword),
				pgdriver.WithDatabase(config.DatabaseName),
			)
	} else {
		pgOptions = pgdriver.NewConnector(
			pgdriver.WithAddr(config.DatabaseEndpointURL),
			pgdriver.WithUser(config.DatabaseUserName),
			pgdriver.WithInsecure(true),
			pgdriver.WithPassword(config.DatabasePassword),
			pgdriver.WithDatabase(config.DatabaseName),
		)
	}

	config.Logger.Debug().Msg(fmt.Sprintf("creating database client with options %+v", pgOptions.Config()))

	// connect to the database
	sqldb := sql.OpenDB(pgOptions)

	db := bun.NewDB(sqldb, pgdialect.New())

	// set up logging on database if requested
	if config.QueryLoggingEnabled {
		db.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	return PostgresClient{
		DBConnection: db,
	}, nil
}

// HealthCheck returns an error if the database can not
// be connected to and queried, nil otherwise
func (pg *PostgresClient) HealthCheck() error {
	_, err := pg.DBConnection.Query(`SELECT 1;`)
	return err
}

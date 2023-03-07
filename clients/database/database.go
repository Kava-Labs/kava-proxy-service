package database

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// Migrate sets up and runs all migrations in the migrations model
// that haven't been run on the database being used by the proxy service
// returning error (if any) and a list of migrations that have been
// run and any that were not
func Migrate(ctx context.Context, db *bun.DB, migrations migrate.Migrations) (*migrate.MigrationSlice, error) {
	// set up migration config
	migrator := migrate.NewMigrator(db, &migrations)

	// create / verify tables used to tack migrations
	err := migrator.Init(ctx)

	if err != nil {
		return &migrate.MigrationSlice{}, err
	}

	// run all un-applied migrations
	group, err := migrator.Migrate(ctx)

	// if migration failed attempt to rollback so migrations can be re-attempted
	if err != nil {
		group, rollbackErr := migrator.Rollback(ctx)

		if rollbackErr != nil {
			return &migrate.MigrationSlice{}, fmt.Errorf("error %s rolling back after original error %s", rollbackErr, err)
		}

		if group.ID == 0 {
			return &migrate.MigrationSlice{}, fmt.Errorf("no groups to rollback after migration error %s", err)
		}

		return &migrate.MigrationSlice{}, fmt.Errorf("rolled back after migration error %s", err)
	}

	// get the status of all run and un-run migrations
	ms, err := migrator.MigrationsWithStatus(ctx)

	if err != nil {
		return &migrate.MigrationSlice{}, err
	}

	if group.ID == 0 {
		fmt.Printf("there are no new migrations to run\n")
	}

	return &ms, nil
}

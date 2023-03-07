// when this package is loaded the package directory
// will be searched for any go or sql migration files and
// register for them to the proxy service to run
// https://bun.uptrace.dev/guide/migrations.html#go-based-migrations
package migrations

import "github.com/uptrace/bun/migrate"

var Migrations = migrate.NewMigrations()

func init() {
	if err := Migrations.DiscoverCaller(); err != nil {
		panic(err)
	}
}

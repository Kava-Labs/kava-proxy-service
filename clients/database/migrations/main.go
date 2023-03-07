// when this package is loaded the package directory
// will be searched for any go or sql migration files and
// register for them to the proxy service to run
// https://bun.uptrace.dev/guide/migrations.html#go-based-migrations
package migrations

import (
	"embed"

	"github.com/uptrace/bun/migrate"
)

//go:embed *.sql
var SQLMigrations embed.FS

var Migrations = migrate.NewMigrations()

func init() {
	if err := Migrations.Discover(SQLMigrations); err != nil {
		panic(err)
	}
}

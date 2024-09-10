package database

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/migrate"
	"testing"
)

func TestMigrateNoDatabase(t *testing.T) {
	migrations, err := Migrate(context.Background(), nil, migrate.Migrations{}, nil)
	require.NoError(t, err)
	require.Empty(t, migrations)
}

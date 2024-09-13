package postgres

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/migrate"
	"testing"
)

func TestMigrateNoDatabase(t *testing.T) {
	db := &Client{}

	_, err := db.Migrate(context.Background(), migrate.Migrations{}, nil)
	require.Error(t, err)
}

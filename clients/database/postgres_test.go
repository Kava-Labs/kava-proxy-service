package database

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDisabledDBCreation(t *testing.T) {
	config := PostgresDatabaseConfig{
		DatabaseDisabled: true,
	}
	db, err := NewPostgresClient(config)
	require.NoError(t, err)
	require.True(t, db.isDisabled)
}

func TestHealthcheckNoDatabase(t *testing.T) {
	config := PostgresDatabaseConfig{
		DatabaseDisabled: true,
	}
	db, err := NewPostgresClient(config)
	require.NoError(t, err)
	err = db.HealthCheck()
	require.NoError(t, err)
}

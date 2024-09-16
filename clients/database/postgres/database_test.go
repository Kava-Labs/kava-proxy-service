package postgres

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDisabledDBCreation(t *testing.T) {
	config := DatabaseConfig{}
	_, err := NewClient(config)
	require.Error(t, err)
}

func TestHealthcheckNoDatabase(t *testing.T) {
	config := DatabaseConfig{}
	_, err := NewClient(config)
	require.Error(t, err)
}

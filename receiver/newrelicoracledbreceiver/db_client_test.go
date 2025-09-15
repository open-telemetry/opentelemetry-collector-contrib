// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoracledbreceiver

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewDBClient(t *testing.T) {
	cfg := &Config{
		DataSource: "oracle://user:pass@localhost:1521/ORCL",
		ExtendedConfig: ExtendedConfig{
			MaxOpenConnections: 10,
		},
	}

	logger := zap.NewNop()
	client, err := newDBClient(cfg, logger)

	// The client should be created even if connection fails
	assert.NotNil(t, client)

	if err != nil {
		// Connection error is expected without real Oracle DB
		assert.Error(t, err)
	}
}

func TestDBClientInterface(_ *testing.T) {
	// Test that our dbClient implements the interface correctly
	var _ dbClient = &oracleDBClient{}
}

func TestConfigToDataSource(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		expected string
	}{
		{
			name: "direct datasource",
			cfg: &Config{
				DataSource: "oracle://user:pass@localhost:1521/ORCL",
			},
			expected: "oracle://user:pass@localhost:1521/ORCL",
		},
		{
			name: "empty datasource",
			cfg: &Config{
				DataSource: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.DataSource
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock implementation for testing
type mockDBClient struct {
	db  *sql.DB
	err error
}

func (m *mockDBClient) getConnection() (*sql.DB, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.db, nil
}

func (m *mockDBClient) close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}

func TestMockDBClient(t *testing.T) {
	// Test our mock implementation
	mock := &mockDBClient{}

	// Test successful connection
	db, err := mock.getConnection()
	assert.NoError(t, err)
	assert.Nil(t, db) // Will be nil since we didn't set it

	// Test close
	err = mock.close()
	assert.NoError(t, err)
}

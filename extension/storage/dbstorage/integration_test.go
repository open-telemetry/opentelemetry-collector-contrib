// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package dbstorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

func TestExtensionIntegrityWithPostgres(t *testing.T) {
	if runtime.GOOS == "windows" && os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping test on Windows GH runners: test requires Docker to be running Linux containers")
	}

	se, ctr, err := newPostgresTestExtension()
	t.Cleanup(func() {
		if ctr != nil {
			require.NoError(t, ctr.Terminate(context.Background())) //nolint:usetesting
		}
	})
	require.NoError(t, err)

	testExtensionIntegrity(t, se)
}

func newPostgresTestExtension() (storage.Extension, testcontainers.Container, error) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "postgres:14",
			// Expose the Postgres port and let Docker assign a free host port
			// (retrieved below via MappedPort). Binding to a fixed host port
			// makes the test fail when 5432 is already in use on the host.
			ExposedPorts: []string{"5432"},
			Env: map[string]string{
				"POSTGRES_PASSWORD": "passwd",
				"POSTGRES_USER":     "root",
				"POSTGRES_DB":       "db",
			},
			WaitingFor: wait.ForListeningPort("5432"),
		},
		Started: true,
	}

	ctr, err := testcontainers.GenericContainer(context.Background(), req)
	if err != nil {
		return nil, nil, err
	}
	port, err := ctr.MappedPort(context.Background(), "5432")
	if err != nil {
		return nil, nil, err
	}
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.DriverName = driverPostgreSQL
	cfg.DataSource = fmt.Sprintf("host=%s port=%s user=%s password=%s database=%s sslmode=disable", "127.0.0.1", port.Port(), "root", "passwd", "db")

	extension, err := f.Create(context.Background(), extensiontest.NewNopSettings(f.Type()), cfg)
	if err != nil {
		return nil, nil, err
	}

	se, ok := extension.(storage.Extension)
	if !ok {
		return nil, nil, errors.New("created extension is not a storage extension")
	}

	return se, ctr, nil
}

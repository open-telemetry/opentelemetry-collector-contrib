// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "localhost:3306"
	expected.Username = "otel"
	expected.Password = "${env:MYSQL_PASSWORD}"
	expected.Database = "otel"
	expected.CollectionInterval = 10 * time.Second
	// This defaults to true when tls is omitted from the configmap.
	expected.TLS.Insecure = true

	require.Equal(t, expected, cfg)
}

func TestLoadConfigDefaultTLS(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String() + "/default_tls")
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "localhost:3306"
	expected.Username = "otel"
	expected.Password = "${env:MYSQL_PASSWORD}"
	expected.Database = "otel"
	expected.CollectionInterval = 10 * time.Second
	// This defaults to false when tls is defined in the configmap.
	expected.TLS.Insecure = false
	expected.TLS.ServerName = "localhost"

	require.Equal(t, expected, cfg)
}

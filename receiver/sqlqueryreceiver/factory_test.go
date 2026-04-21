// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlqueryreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Driver = "postgres"
	_, err := factory.CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	_, err = factory.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
}

func TestNewFactoryDataSourceFields(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Driver = "postgres"
	cfg.Host = "localhost"
	cfg.Port = 5432
	cfg.Database = "test"
	cfg.Username = "test"
	cfg.Password = "test"

	cfg.Queries = []sqlquery.Query{
		{
			SQL: "SELECT 1",
			Metrics: []sqlquery.MetricCfg{
				{
					MetricName:  "test_metric",
					ValueColumn: "1",
					ValueType:   sqlquery.MetricValueTypeInt,
					DataType:    sqlquery.MetricTypeGauge,
				},
			},
		},
	}

	_, err := factory.CreateMetrics(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	_, err = factory.CreateLogs(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NoError(t, err)
}

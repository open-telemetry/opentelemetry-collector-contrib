// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package slowsqlconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/slowsqlconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/slowsqlconnector/internal/metadata"
)

// NewFactory creates a factory for the slowsql connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogsConnector, metadata.TracesToLogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Threshold: time.Millisecond * 500,
		DBSystem: []string{
			conventions.DBSystemH2.Value.AsString(), conventions.DBSystemMongoDB.Value.AsString(),
			conventions.DBSystemMySQL.Value.AsString(), conventions.DBSystemOracle.Value.AsString(),
			conventions.DBSystemPostgreSQL.Value.AsString(), conventions.DBSystemMariaDB.Value.AsString(),
		},
		Dimensions: []Dimension{},
	}
}

func createTracesToLogsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Traces, error) {
	lc := newLogsConnector(params.Logger, cfg)
	lc.logsConsumer = nextConsumer
	return lc, nil
}

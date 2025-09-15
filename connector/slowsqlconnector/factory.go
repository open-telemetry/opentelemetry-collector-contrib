// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package slowsqlconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/slowsqlconnector"

import (
	"context"
	"time"

	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/slowsqlconnector/internal/metadata"
)

// NewFactory creates a factory for the exceptions connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogsConnector, metadata.TracesToLogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Threshold: time.Microsecond * 500,
		DBSystem: []string{
			string(conventions.DBSystemH2.Key), string(conventions.DBSystemMongoDB.Key),
			string(conventions.DBSystemMySQL.Key), string(conventions.DBSystemOracle.Key),
			string(conventions.DBSystemPostgreSQL.Key), string(conventions.DBSystemMariaDB.Key),
		},
		Dimensions: []Dimension{},
	}
}

func createTracesToLogsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Traces, error) {
	lc := newLogsConnector(params.Logger, cfg)
	lc.logsConsumer = nextConsumer
	return lc, nil
}

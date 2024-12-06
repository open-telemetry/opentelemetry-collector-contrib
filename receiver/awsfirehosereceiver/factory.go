// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/auto"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/cwmetricstream"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/otlpmetricstream"
)

const (
	defaultEndpoint = "0.0.0.0:4433"
	defaultPort     = 4433
)

var (
	errUnrecognizedRecordType = errors.New("unrecognized record type")
	availableRecordTypes      = map[string]bool{
		cwmetricstream.TypeStr:   true,
		cwlog.TypeStr:            true,
		otlpmetricstream.TypeStr: true,
		auto.TypeStr:             true,
	}
)

// NewFactory creates a receiver factory for awsfirehose. Currently, only
// available in metrics pipelines.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

// validateRecordType checks the available record types for the
// passed in one and returns an error if not found.
func validateRecordType(recordType string) error {
	if _, ok := availableRecordTypes[recordType]; !ok {
		return errUnrecognizedRecordType
	}
	return nil
}

// defaultMetricsUnmarshalers creates a map of the available metrics
// unmarshalers.
func defaultMetricsUnmarshalers(logger *zap.Logger) map[string]unmarshaler.MetricsUnmarshaler {
	cwmsu := cwmetricstream.NewUnmarshaler(logger)
	otlpv1msu := otlpmetricstream.NewUnmarshaler(logger)
	return map[string]unmarshaler.MetricsUnmarshaler{
		cwmsu.Type():     cwmsu,
		otlpv1msu.Type(): otlpv1msu,
	}
}

// defaultLogsUnmarshalers creates a map of the available logs unmarshalers.
func defaultLogsUnmarshalers(logger *zap.Logger) map[string]unmarshaler.LogsUnmarshaler {
	u := cwlog.NewUnmarshaler(logger)
	return map[string]unmarshaler.LogsUnmarshaler{
		u.Type(): u,
	}
}

// createDefaultConfig creates a default config with the endpoint set
// to port 8443 and the record type set to the CloudWatch metric stream.
func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: testutil.EndpointForPort(defaultPort),
		},
	}
}

// createMetricsReceiver implements the CreateMetrics function type.
func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	return newMetricsReceiver(cfg.(*Config), set, defaultMetricsUnmarshalers(set.Logger), nextConsumer)
}

// createMetricsReceiver implements the CreateMetricsReceiver function type.
func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	return newLogsReceiver(cfg.(*Config), set, defaultLogsUnmarshalers(set.Logger), nextConsumer)
}

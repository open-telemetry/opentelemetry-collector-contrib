// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

// This file implements factory for Splunk HEC receiver.

const (
	// Default endpoint to bind to.
	defaultEndpoint = "localhost:8088"
)

// NewFactory creates a factory for Splunk HEC receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

// CreateDefaultConfig creates the default configuration for Splunk HEC receiver.
func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: defaultEndpoint,
		},
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{},
		HecToOtelAttrs: translator.HecToOtelAttrs{
			Source:     splunk.DefaultSourceLabel,
			SourceType: splunk.DefaultSourceTypeLabel,
			Index:      splunk.DefaultIndexLabel,
			Host:       string(conventions.HostNameKey),
		},
		RawPath:    splunk.DefaultRawPath,
		HealthPath: splunk.DefaultHealthPath,
		Ack: Ack{
			Extension: nil,
			Path:      splunk.DefaultAckPath,
		},
		Splitting: SplittingStrategyLine,
	}
}

// CreateMetrics creates a metrics receiver based on provided config.
func createMetricsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	var err error
	var recv receiver.Metrics
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newReceiver(params, *rCfg)
		return recv
	})
	if err != nil {
		return nil, err
	}
	r.Unwrap().(*splunkReceiver).metricsConsumer = consumer
	return r, nil
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.Settings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	var err error
	var recv receiver.Logs
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newReceiver(params, *rCfg)
		return recv
	})
	if err != nil {
		return nil, err
	}
	r.Unwrap().(*splunkReceiver).logsConsumer = consumer
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()

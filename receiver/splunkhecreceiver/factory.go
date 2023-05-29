// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
)

// This file implements factory for Splunk HEC receiver.

const (
	// Default endpoints to bind to.
	defaultEndpoint = ":8088"
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
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: defaultEndpoint,
		},
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{},
		HecToOtelAttrs: splunk.HecToOtelAttrs{
			Source:     splunk.DefaultSourceLabel,
			SourceType: splunk.DefaultSourceTypeLabel,
			Index:      splunk.DefaultIndexLabel,
			Host:       conventions.AttributeHostName,
		},
		RawPath:    splunk.DefaultRawPath,
		HealthPath: splunk.DefaultHealthPath,
	}
}

// CreateMetricsReceiver creates a metrics receiver based on provided config.
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	var err error
	var recv receiver.Metrics
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newMetricsReceiver(params, *rCfg, consumer)
		return recv
	})
	if err != nil {
		return nil, err
	}
	r.Component.(*splunkReceiver).metricsConsumer = consumer
	return r, nil
}

// createLogsReceiver creates a logs receiver based on provided config.
func createLogsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	cfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	var err error
	var recv receiver.Logs
	rCfg := cfg.(*Config)
	r := receivers.GetOrAdd(cfg, func() component.Component {
		recv, err = newLogsReceiver(params, *rCfg, consumer)
		return recv
	})
	if err != nil {
		return nil, err
	}
	r.Component.(*splunkReceiver).logsConsumer = consumer
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()

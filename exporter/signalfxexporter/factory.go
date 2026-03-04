// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
)

const (
	defaultHTTPTimeout          = time.Second * 10
	defaultHTTP2ReadIdleTimeout = time.Second * 10
	defaultHTTP2PingTimeout     = time.Second * 10
	defaultMaxConns             = 100

	defaultDimMaxBuffered         = 10000
	defaultDimSendDelay           = 10 * time.Second
	defaultDimMaxConnsPerHost     = 20
	defaultDimMaxIdleConns        = 20
	defaultDimMaxIdleConnsPerHost = 20
)

// NewFactory creates a factory for SignalFx exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	maxConnCount := defaultMaxConns
	idleConnTimeout := 30 * time.Second
	timeout := 10 * time.Second
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Timeout = defaultHTTPTimeout
	clientConfig.MaxIdleConns = maxConnCount
	clientConfig.MaxIdleConnsPerHost = maxConnCount
	clientConfig.IdleConnTimeout = idleConnTimeout
	clientConfig.HTTP2ReadIdleTimeout = defaultHTTP2ReadIdleTimeout
	clientConfig.HTTP2PingTimeout = defaultHTTP2PingTimeout

	return &Config{
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		ClientConfig:  clientConfig,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
		DeltaTranslationTTL:           3600,
		Correlation:                   correlation.DefaultConfig(),
		NonAlphanumericDimensionChars: "_-.",
		DimensionClient: DimensionClientConfig{
			SendDelay:           defaultDimSendDelay,
			MaxBuffered:         defaultDimMaxBuffered,
			MaxConnsPerHost:     defaultDimMaxConnsPerHost,
			MaxIdleConns:        defaultDimMaxIdleConns,
			MaxIdleConnsPerHost: defaultDimMaxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeout,
			Timeout:             timeout,
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	eCfg component.Config,
) (exporter.Traces, error) {
	cfg := eCfg.(*Config)
	corrCfg := cfg.Correlation

	if corrCfg.Endpoint == "" {
		apiURL, err := cfg.getAPIURL()
		if err != nil {
			return nil, fmt.Errorf("unable to create API URL: %w", err)
		}
		corrCfg.Endpoint = apiURL.String()
	}
	if cfg.AccessToken == "" {
		return nil, errors.New("access_token is required")
	}
	set.Logger.Info("Correlation tracking enabled", zap.String("endpoint", corrCfg.Endpoint))
	tracker := correlation.NewTracker(corrCfg, cfg.AccessToken, set)

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		tracker.ProcessTraces,
		exporterhelper.WithStart(tracker.Start),
		exporterhelper.WithShutdown(tracker.Shutdown))
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg := config.(*Config)

	exp, err := newSignalFxExporter(cfg, set)
	if err != nil {
		return nil, err
	}

	me, err := exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exp.pushMetrics,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.shutdown))
	if err != nil {
		return nil, err
	}

	// If AccessTokenPassthrough enabled, split the incoming Metrics data by splunk.SFxAccessTokenLabel,
	// this ensures that we get batches of data for the same token when pushing to the backend.
	if cfg.AccessTokenPassthrough {
		me = &baseMetricsExporter{
			Component: me,
			Metrics:   batchperresourceattr.NewBatchPerResourceMetrics(splunk.SFxAccessTokenLabel, me),
		}
	}

	return &signalfMetadataExporter{
		Metrics:  me,
		exporter: exp,
	}, nil
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	expCfg := cfg.(*Config)

	exp, err := newEventExporter(expCfg, set)
	if err != nil {
		return nil, err
	}

	le, err := exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		exp.pushLogs,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.BackOffConfig),
		exporterhelper.WithQueue(expCfg.QueueSettings),
		exporterhelper.WithStart(exp.startLogs))
	if err != nil {
		return nil, err
	}

	// If AccessTokenPassthrough enabled, split the incoming Metrics data by splunk.SFxAccessTokenLabel,
	// this ensures that we get batches of data for the same token when pushing to the backend.
	if expCfg.AccessTokenPassthrough {
		le = &baseLogsExporter{
			Component: le,
			Logs:      batchperresourceattr.NewBatchPerResourceLogs(splunk.SFxAccessTokenLabel, le),
		}
	}

	return le, nil
}

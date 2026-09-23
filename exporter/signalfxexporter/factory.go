// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"

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
	clientConfig.MaxIdleConns = maxConnCount        //nolint:staticcheck // SA1019: deprecated field still used for default config value; see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316
	clientConfig.MaxIdleConnsPerHost = maxConnCount //nolint:staticcheck // SA1019: deprecated field still used for default config value; see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316
	clientConfig.IdleConnTimeout = idleConnTimeout  //nolint:staticcheck // SA1019: deprecated field still used for default config value; see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316
	clientConfig.HTTP2ReadIdleTimeout = defaultHTTP2ReadIdleTimeout
	clientConfig.HTTP2PingTimeout = defaultHTTP2PingTimeout

	return &Config{
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		ClientConfig:  clientConfig,
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
		DeltaTranslationTTL:           3600,
		NonAlphanumericDimensionChars: "_-.",
		DimensionClient: DimensionClientConfig{
			SendDelay:           defaultDimSendDelay,
			MaxBuffered:         defaultDimMaxBuffered,
			MaxConnsPerHost:     defaultDimMaxConnsPerHost,
			MaxIdleConns:        defaultDimMaxIdleConns,
			MaxIdleConnsPerHost: defaultDimMaxIdleConnsPerHost,
			IdleConnTimeout:     idleConnTimeout,
			Timeout:             timeout,
			StripK8sLabelPrefix: true,
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	eCfg component.Config,
) (exporter.Traces, error) {
	return exporterhelper.NewTraces(
		ctx,
		set,
		eCfg,
		noOpProcessTraces,
	)
}

func noOpProcessTraces(_ context.Context, _ ptrace.Traces) error {
	return nil
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
		exporterhelper.WithShutdown(exp.shutdown),
	)
	if err != nil {
		return nil, err
	}

	// If AccessTokenPassthrough enabled, split the incoming Metrics data by splunk.SFxAccessTokenLabel,
	// this ensures that we get batches of data for the same token when pushing to the backend.
	if cfg.AccessTokenPassthroughConfig.AccessTokenPassthrough {
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
		exporterhelper.WithStart(exp.startLogs),
		exporterhelper.WithShutdown(exp.shutdown),
	)
	if err != nil {
		return nil, err
	}

	// If AccessTokenPassthrough enabled, split the incoming Metrics data by splunk.SFxAccessTokenLabel,
	// this ensures that we get batches of data for the same token when pushing to the backend.
	if expCfg.AccessTokenPassthroughConfig.AccessTokenPassthrough {
		le = &baseLogsExporter{
			Component: le,
			Logs:      batchperresourceattr.NewBatchPerResourceLogs(splunk.SFxAccessTokenLabel, le),
		}
	}

	return le, nil
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

const (
	// The value of "type" key in configuration.
	defaultLogsIndex   = "logs-generic-default"
	defaultTracesIndex = "traces-generic-default"
	userAgentHeaderKey = "User-Agent"
)

// NewFactory creates a factory for Elastic exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      exporterhelper.NewDefaultQueueSettings().Enabled,
			NumConsumers: 100,
			QueueSize:    exporterhelper.NewDefaultQueueSettings().QueueSize,
		},
		ClientConfig: ClientConfig{
			Timeout: 90 * time.Second,
		},
		Index:       "",
		LogsIndex:   defaultLogsIndex,
		TracesIndex: defaultTracesIndex,
		Retry: RetrySettings{
			Enabled:         true,
			MaxRequests:     3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
			RetryOnStatus: []int{
				http.StatusTooManyRequests,
				http.StatusInternalServerError,
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
				http.StatusGatewayTimeout,
			},
		},
		BatcherConfig: exporterbatcher.Config{
			Enabled:      true,
			FlushTimeout: 30 * time.Second,
			MinSizeConfig: exporterbatcher.MinSizeConfig{
				MinSizeItems: 125,
			},
			MaxSizeConfig: exporterbatcher.MaxSizeConfig{},
		},
		Mapping: MappingsSettings{
			Mode:  "none",
			Dedup: true,
			Dedot: true,
		},
		LogstashFormat: LogstashFormatSettings{
			Enabled:         false,
			PrefixSeparator: "-",
			DateFormat:      "%Y.%m.%d",
		},
		NumWorkers: runtime.NumCPU(),
	}
}

// createLogsExporter creates a new exporter for logs.
//
// Logs are directly indexed into Elasticsearch.
func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	cf := cfg.(*Config)

	setDefaultUserAgentHeader(cf, set.BuildInfo)

	if err := handleDeprecations(cf, set.Logger); err != nil {
		return nil, err
	}

	exporter, err := newExporter(set.Logger, cf, cf.LogsIndex, cf.LogsDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.pushLogsData,
		exporterhelper.WithBatcher(cf.BatcherConfig),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings),
		exporterhelper.WithTimeout(getTimeoutConfig()),
	)
}

// createTracesExporter creates a new exporter for traces.
func createTracesExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config) (exporter.Traces, error) {

	cf := cfg.(*Config)

	setDefaultUserAgentHeader(cf, set.BuildInfo)

	if err := handleDeprecations(cf, set.Logger); err != nil {
		return nil, err
	}

	exporter, err := newExporter(set.Logger, cf, cf.TracesIndex, cf.TracesDynamicIndex.Enabled)
	if err != nil {
		return nil, fmt.Errorf("cannot configure Elasticsearch exporter: %w", err)
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exporter.pushTraceData,
		exporterhelper.WithBatcher(cf.BatcherConfig),
		exporterhelper.WithShutdown(exporter.Shutdown),
		exporterhelper.WithQueue(cf.QueueSettings),
		exporterhelper.WithTimeout(getTimeoutConfig()),
	)
}

// set default User-Agent header with BuildInfo if User-Agent is empty
func setDefaultUserAgentHeader(cf *Config, info component.BuildInfo) {
	if _, found := cf.Headers[userAgentHeaderKey]; found {
		return
	}
	if cf.Headers == nil {
		cf.Headers = make(map[string]string)
	}
	cf.Headers[userAgentHeaderKey] = fmt.Sprintf("%s/%s (%s/%s)", info.Description, info.Version, runtime.GOOS, runtime.GOARCH)
}

func getTimeoutConfig() exporterhelper.TimeoutSettings {
	return exporterhelper.TimeoutSettings{
		Timeout: time.Duration(0), // effectively disable timeout_sender because timeout is enforced in bulk indexer
	}
}

// handleDeprecations handles deprecated config options.
// If possible, translate deprecated config options to new config options
// Otherwise, return an error so that the user is aware of an unsupported option.
func handleDeprecations(cf *Config, logger *zap.Logger) error {
	if cf.Index != "" {
		logger.Warn(`"index" option is deprecated and replaced with "logs_index" and "traces_index". "logs_index" is set to the value of "index".`)
		cf.LogsIndex = cf.Index
	}

	if cf.Flush.Bytes != 0 {
		// cannot translate flush.bytes to batcher.min_size_items because they are in different units
		return errors.New(`"flush.bytes" option is unsupported, use "batcher.min_size_items" instead`)
	}

	if cf.Flush.Interval != 0 {
		logger.Warn(`"flush.interval" option is deprecated and replaced with "batcher.flush_timeout". "batcher.flush_timeout" is set to the value of "flush.interval".`)
		cf.BatcherConfig.FlushTimeout = cf.Flush.Interval
	}

	return nil
}

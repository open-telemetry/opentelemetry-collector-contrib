// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/correlation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
)

const (
	// The value of "type" key in configuration.
	typeStr = "signalfx"

	defaultHTTPTimeout = time.Second * 5
)

// NewFactory creates a factory for SignalFx exporter.
func NewFactory() component.ExporterFactory {
	return component.NewExporterFactory(
		typeStr,
		createDefaultConfig,
		component.WithMetricsExporter(createMetricsExporter),
		component.WithLogsExporter(createLogsExporter),
		component.WithTracesExporter(createTracesExporter),
	)
}

func createDefaultConfig() config.Exporter {
	return &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		TimeoutSettings:  exporterhelper.TimeoutSettings{Timeout: defaultHTTPTimeout},
		RetrySettings:    exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:    exporterhelper.NewDefaultQueueSettings(),
		AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: true,
		},
		DeltaTranslationTTL:           3600,
		Correlation:                   correlation.DefaultConfig(),
		NonAlphanumericDimensionChars: "_-.",
		MaxConnections:                100,
	}
}

func createTracesExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	eCfg config.Exporter,
) (component.TracesExporter, error) {
	cfg := eCfg.(*Config)
	corrCfg := cfg.Correlation

	if corrCfg.Endpoint == "" {
		apiURL, err := cfg.getAPIURL()
		if err != nil {
			return nil, fmt.Errorf("unable to create API URL: %v", err)
		}
		corrCfg.Endpoint = apiURL.String()
	}
	if cfg.AccessToken == "" {
		return nil, errors.New("access_token is required")
	}
	set.Logger.Info("Correlation tracking enabled", zap.String("endpoint", corrCfg.Endpoint))
	tracker := correlation.NewTracker(corrCfg, cfg.AccessToken, set)

	return exporterhelper.NewTracesExporter(
		cfg,
		set,
		tracker.AddSpans,
		exporterhelper.WithStart(tracker.Start),
		exporterhelper.WithShutdown(tracker.Shutdown))
}

func createMetricsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	config config.Exporter,
) (component.MetricsExporter, error) {

	expCfg := config.(*Config)

	err := setDefaultExcludes(expCfg)
	if err != nil {
		return nil, err
	}

	exp, err := newSignalFxExporter(expCfg, set.Logger)
	if err != nil {
		return nil, err
	}

	me, err := exporterhelper.NewMetricsExporter(
		expCfg,
		set,
		exp.pushMetrics,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings))

	if err != nil {
		return nil, err
	}

	// If AccessTokenPassthrough enabled, split the incoming Metrics data by splunk.SFxAccessTokenLabel,
	// this ensures that we get batches of data for the same token when pushing to the backend.
	if expCfg.AccessTokenPassthrough {
		me = &baseMetricsExporter{
			Component: me,
			Metrics:   batchperresourceattr.NewBatchPerResourceMetrics(splunk.SFxAccessTokenLabel, me),
		}
	}

	return &signalfMetadataExporter{
		MetricsExporter: me,
		pushMetadata:    exp.pushMetadata,
	}, nil
}

func loadDefaultTranslationRules() ([]translation.Rule, error) {
	cfg, err := loadConfig([]byte(translation.DefaultTranslationRulesYaml))
	return cfg.TranslationRules, err
}

// setDefaultExcludes appends default metrics to be excluded to the exclude_metrics option.
func setDefaultExcludes(cfg *Config) error {
	defaultExcludeMetrics, err := loadDefaultExcludes()
	if err != nil {
		return err
	}
	if cfg.ExcludeMetrics == nil || len(cfg.ExcludeMetrics) > 0 {
		cfg.ExcludeMetrics = append(cfg.ExcludeMetrics, defaultExcludeMetrics...)
	}
	return nil
}

func loadDefaultExcludes() ([]dpfilters.MetricFilter, error) {
	cfg, err := loadConfig([]byte(translation.DefaultExcludeMetricsYaml))
	return cfg.ExcludeMetrics, err
}

func loadConfig(bytes []byte) (Config, error) {
	var cfg Config
	var data map[string]interface{}
	if err := yaml.Unmarshal(bytes, &data); err != nil {
		return cfg, err
	}

	if err := config.NewMapFromStringMap(data).UnmarshalExact(&cfg); err != nil {
		return cfg, fmt.Errorf("failed to load default exclude metrics: %v", err)
	}

	return cfg, nil
}

func createLogsExporter(
	_ context.Context,
	set component.ExporterCreateSettings,
	cfg config.Exporter,
) (component.LogsExporter, error) {
	expCfg := cfg.(*Config)

	exp, err := newEventExporter(expCfg, set.Logger)
	if err != nil {
		return nil, err
	}

	le, err := exporterhelper.NewLogsExporter(
		expCfg,
		set,
		exp.pushLogs,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(expCfg.RetrySettings),
		exporterhelper.WithQueue(expCfg.QueueSettings))

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

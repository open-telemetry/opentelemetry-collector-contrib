// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"
import (
	"context"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"sync"
	"time"
)

var onceMetrics sync.Once

// NewFactory returns a new factory for the Delta to Rate processor.
func NewFactory() processor.Factory {
	onceMetrics.Do(func() {
		// TODO: this is hardcoding the metrics level and skips error handling
		_ = view.Register(deltaToCumulativeProcessorMetricViews(configtelemetry.LevelNormal)...)
	})
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		MaxStaleness: time.Duration(180) * time.Minute,
		SendInterval: time.Duration(30) * time.Second,
		IdentifyMode: ScopeAndResourceAttributes,
		Level:        configtelemetry.LevelNormal,
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}
	telemetrySettings := &component.TelemetrySettings{
		Logger:        set.Logger,
		MeterProvider: set.MeterProvider,
		MetricsLevel:  set.MetricsLevel,
		Resource:      set.Resource,
	}

	metricsProcessor := newDeltaToCumulativeProcessor(ctx, processorConfig, nextConsumer, telemetrySettings, false)

	return metricsProcessor, nil
}

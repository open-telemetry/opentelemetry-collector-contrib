package awsecsattributesprocessor

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/xprocessor"
)

// Factory, builds a new instance of the component

const (
	// The value of "type" key in configuration.
	typeStr = "ecsattributesprocessor"
)

var (
	componentStability = component.StabilityLevelAlpha
)

// NewFactory creates a factory for the routing processor.
func NewFactory() processor.Factory {
	return xprocessor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xprocessor.WithLogs(createLogsProcessor, componentStability),
		xprocessor.WithMetrics(createMetricsProcessor, componentStability),
		xprocessor.WithTraces(createTracesProcessor, componentStability),
		xprocessor.WithProfiles(createProfilesProcessor, componentStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		CacheTTL: 300, // 5 minutes default
		Attributes: []string{
			// by default, we collect all tribute nameaå that start with:
			// ecs, name, image or docker
			"^aws.ecs.*|^image.*|^docker.*|^labels.*",
		},
	}
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (processor.Logs, error) {

	// create logger
	logger := set.TelemetrySettings.Logger

	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", typeStr)
	}

	// initialise config
	if err := config.init(); err != nil {
		return nil, err
	}

	return newLogsProcessor(
		ctx,
		logger,
		config,
		nextConsumer,
		getEndpoints,
		getContainerData,
	), nil
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {

	// create logger
	logger := set.TelemetrySettings.Logger

	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", typeStr)
	}

	// initialise config
	if err := config.init(); err != nil {
		return nil, err
	}

	return newMetricsProcessor(
		ctx,
		logger,
		config,
		nextConsumer,
		getEndpoints,
		getContainerData,
	), nil
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {

	// create logger
	logger := set.TelemetrySettings.Logger

	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", typeStr)
	}

	// initialise config
	if err := config.init(); err != nil {
		return nil, err
	}

	return newTracesProcessor(
		ctx,
		logger,
		config,
		nextConsumer,
		getEndpoints,
		getContainerData,
	), nil
}

func createProfilesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xprocessor.Profiles, error) {

	// create logger
	logger := set.TelemetrySettings.Logger

	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config for processor %s", typeStr)
	}

	// initialise config
	if err := config.init(); err != nil {
		return nil, err
	}

	return newProfilesProcessor(
		ctx,
		logger,
		config,
		nextConsumer,
		getEndpoints,
		getContainerData,
	), nil
}

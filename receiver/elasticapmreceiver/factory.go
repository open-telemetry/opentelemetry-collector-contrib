package elasticapmreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	typeStr                  = "elasticapm"
	defaultHTTPEndpoint      = "0.0.0.0:8200"
	defaultEventsURLPath     = "/intake/v2/events"
	defaultRUMEventsURLPath  = "/intake/v2/rum/events"
	defaultMaxEventSizeBytes = 300 * 1024
	defaultBatchSize         = 10
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		receiver.WithTraces(createTraces, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: &confighttp.ServerConfig{
			Endpoint: defaultHTTPEndpoint,
		},
		EventsURLPath:    defaultEventsURLPath,
		RUMEventsUrlPath: defaultRUMEventsURLPath,
		MaxEventSize:     defaultMaxEventSizeBytes,
		BatchSize:        defaultBatchSize,
	}
}

func createTraces(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	cfg := baseCfg.(*Config)
	r, err := newElasticAPMReceiver(cfg, params)

	if err != nil {
		return nil, err
	}

	if err = r.registerTraceConsumer(nextConsumer); err != nil {
		return nil, err
	}

	return r, nil
}

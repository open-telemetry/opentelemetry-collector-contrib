package datadogrumreceiver

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/metadata"
)

// NewFactory creates a factory for the Datadog RUM receiver
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(createTracesReceiver, metadata.TracesStability),
		receiver.WithLogs(createLogsReceiver, metadata.LogsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint: "localhost:12722",
		},
		ReadTimeout: 60 * time.Second,
	}
}

func createTracesReceiver(_ context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Traces) (receiver.Traces, error) {
	var err error
	rcfg := cfg.(*Config)
	r := receivers.GetOrAdd(rcfg, func() (dd component.Component) {
		dd, err = newDataDogRUMReceiver(rcfg, params)
		return dd
	})
	if err != nil {
		return nil, err
	}

	r.Unwrap().(*datadogRUMReceiver).nextTracesConsumer = consumer
	return r, nil
}

func createLogsReceiver(_ context.Context, params receiver.Settings, cfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	var err error
	rcfg := cfg.(*Config)
	r := receivers.GetOrAdd(rcfg, func() (dd component.Component) {
		dd, err = newDataDogRUMReceiver(rcfg, params)
		return dd
	})
	if err != nil {
		return nil, err
	}

	r.Unwrap().(*datadogRUMReceiver).nextLogsConsumer = consumer
	return r, nil
}

var receivers = sharedcomponent.NewSharedComponents()

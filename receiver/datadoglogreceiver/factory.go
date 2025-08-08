package datadoglogreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

var (
	typeStr = component.MustNewType("datadoglog")
)

const (
	defaultVersion  = "v2_api"
	defaultEndpoint = "0.0.0.0:10518"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithLogs(createLogsReceiver, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Version: defaultVersion,
		HTTP:    &confighttp.ServerConfig{Endpoint: defaultEndpoint},
	}
}

func createLogsReceiver(_ context.Context, params receiver.Settings, baseCfg component.Config, consumer consumer.Logs) (receiver.Logs, error) {
	cfg := baseCfg.(*Config)

	logReceiver, err := newDatadogLogReceiver(cfg, consumer, params)
	if err != nil {
		return nil, err
	}
	return logReceiver, nil
}

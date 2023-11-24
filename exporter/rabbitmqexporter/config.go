package rabbitmqexporter

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"runtime"
	"time"
)

type config struct {
	connectionUrl               string
	channelPoolSize             int
	connectionTimeout           time.Duration
	publishConfirmationTimeout  time.Duration
	connectionHeartbeatInterval time.Duration
	confirmMode                 bool
	durable                     bool
	routingKey                  string
	retrySettings               exporterhelper.RetrySettings
}

func createDefaultConfig() component.Config {
	retrySettings := exporterhelper.RetrySettings{
		Enabled: false,
	}
	return &config{
		connectionUrl:               "amqp://swar8080amqp:swar8080amqp@localhost:5672/",
		connectionTimeout:           time.Second * 10,
		publishConfirmationTimeout:  time.Second * 5,
		connectionHeartbeatInterval: time.Second * 3,
		channelPoolSize:             runtime.NumCPU(),
		confirmMode:                 true,
		durable:                     true,
		routingKey:                  "otel",
		retrySettings:               retrySettings,
	}
}

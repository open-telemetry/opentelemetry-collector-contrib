module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver

go 1.14

require (

	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	go.opentelemetry.io/collector v0.17.0
	go.uber.org/zap v1.16.0
)

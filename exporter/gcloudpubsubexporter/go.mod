module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/gcloudpubsubexporter

go 1.14

require (
	cloud.google.com/go/pubsub v1.9.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.19.0
	go.uber.org/zap v1.16.0
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20201209185603-f92720507ed4
	google.golang.org/grpc v1.34.0
)

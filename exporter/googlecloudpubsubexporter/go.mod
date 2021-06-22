module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter

go 1.16

require (
	cloud.google.com/go/pubsub v1.11.0
	github.com/google/uuid v1.2.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210630154722-7d0a0398174e
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	google.golang.org/api v0.48.0
	google.golang.org/genproto v0.0.0-20210604141403-392c879c8b08
	google.golang.org/grpc v1.39.0
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210630154722-7d0a0398174e

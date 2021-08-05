module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/docker/docker v20.10.8+incompatible
	github.com/gobwas/glob v0.2.3
	github.com/golang/protobuf v1.5.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210804191544-3cfe4f8c5d3e
	go.uber.org/zap v1.18.1
	google.golang.org/protobuf v1.27.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver => ../redisreceiver

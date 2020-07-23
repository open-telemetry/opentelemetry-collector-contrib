module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver

go 1.13

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/go-redis/redis/v7 v7.2.0
	github.com/golang/protobuf v1.4.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0
	github.com/ory/x v0.0.109 // indirect
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200723232356-d4053cc823a0
	go.uber.org/zap v1.15.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

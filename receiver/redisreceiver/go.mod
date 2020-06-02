module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver

go 1.13

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/go-redis/redis/v7 v7.2.0
	github.com/golang/protobuf v1.3.5
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.3.1-0.20200601172059-a776048b653c
	go.uber.org/zap v1.10.0
)

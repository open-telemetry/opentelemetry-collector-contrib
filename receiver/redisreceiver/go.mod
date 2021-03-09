module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver

go 1.14

require (
	github.com/go-redis/redis/v7 v7.4.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.22.0
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

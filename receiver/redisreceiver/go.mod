module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver

go 1.16

require (
	github.com/go-redis/redis/v7 v7.4.1
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210804191544-3cfe4f8c5d3e
	go.opentelemetry.io/collector/model v0.31.1-0.20210804191544-3cfe4f8c5d3e
	go.uber.org/zap v1.18.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

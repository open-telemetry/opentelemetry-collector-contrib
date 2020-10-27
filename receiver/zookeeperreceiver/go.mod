module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.14.1-0.20201106183657-c6b8f28c60b5
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
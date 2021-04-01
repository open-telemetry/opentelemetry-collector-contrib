module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver

go 1.14

require (
	github.com/go-redis/redis/v7 v7.4.0
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.23.1-0.20210331235005-1edaf40cedaf
	go.uber.org/zap v1.16.0
	gopkg.in/ini.v1 v1.57.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

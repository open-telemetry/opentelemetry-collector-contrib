module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/davecgh/go-spew v1.1.1
	github.com/go-redis/redis/v7 v7.4.0
	github.com/grobie/gomemcache v0.0.0-20180201122607-1f779c573665
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.1-0.20201027215027-6ae66159741d
	go.uber.org/zap v1.16.0
	google.golang.org/api v0.32.0
	google.golang.org/protobuf v1.25.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

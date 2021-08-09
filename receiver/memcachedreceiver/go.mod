module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver

go 1.16

require (
	github.com/grobie/gomemcache v0.0.0-20180201122607-1f779c573665
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210809153342-28acc7d8b7f2
	go.opentelemetry.io/collector/model v0.31.1-0.20210809153342-28acc7d8b7f2
	go.uber.org/zap v1.18.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

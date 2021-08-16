module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator

go 1.16

require (
	github.com/antonmedv/expr v1.9.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0-00010101000000-000000000000
	github.com/spf13/cast v1.4.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210810171211-8038673eba9e
	go.opentelemetry.io/collector/model v0.31.1-0.20210810171211-8038673eba9e
	go.uber.org/zap v1.19.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../../extension/observer

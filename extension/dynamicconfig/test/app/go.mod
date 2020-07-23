module github.com/vmingchen/opentelemetry-collector-contrib/extension/dynamicconfig/test/app

go 1.14

require (
	go.opentelemetry.io/contrib/sdk/dynamicconfig v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v0.7.0
	go.opentelemetry.io/otel/exporters/otlp v0.7.0
)

// replace github.com/open-telemetry/opentelemetry-proto => github.com/vmingchen/opentelemetry-proto v0.3.1-0.20200716191220-7eb25882f08b
replace github.com/open-telemetry/opentelemetry-proto => github.com/vmingchen/opentelemetry-proto v0.3.1-0.20200716191220-7eb25882f08b

replace go.opentelemetry.io/contrib => github.com/vmingchen/opentelemetry-go-contrib v0.0.0-20200717014637-f8d1d2438c26

replace go.opentelemetry.io/contrib/sdk/dynamicconfig => github.com/vmingchen/opentelemetry-go-contrib/sdk/dynamicconfig v0.0.0-20200717014637-f8d1d2438c26

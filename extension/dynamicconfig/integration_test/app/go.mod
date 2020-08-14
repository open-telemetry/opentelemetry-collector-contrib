module github.com/vmingchen/opentelemetry-collector-contrib/extension/dynamicconfig/test/app

go 1.14

require (
	go.opentelemetry.io/contrib/sdk/dynamicconfig v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v0.10.0
	go.opentelemetry.io/otel/exporters/otlp v0.10.0
	go.opentelemetry.io/otel/sdk v0.10.0
)

replace go.opentelemetry.io/contrib => github.com/vmingchen/opentelemetry-go-contrib v0.0.0-20200814193941-35b1c89fca24

replace go.opentelemetry.io/contrib/sdk/dynamicconfig => github.com/vmingchen/opentelemetry-go-contrib/sdk/dynamicconfig v0.0.0-20200814193941-35b1c89fca24

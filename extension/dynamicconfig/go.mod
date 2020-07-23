module github.com/vmingchen/opentelemetry-collector-contrib/extension/dynamicconfig

go 1.14

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-proto v0.4.0
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.0
	go.uber.org/zap v1.13.0
	google.golang.org/grpc v1.29.1
)

// replace github.com/open-telemetry/opentelemetry-proto => github.com/vmingchen/opentelemetry-proto v0.3.1-0.20200716191220-7eb25882f08b
replace github.com/open-telemetry/opentelemetry-proto => github.com/vmingchen/opentelemetry-proto v0.3.1-0.20200716191220-7eb25882f08b

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig => ./

module github.com/vmingchen/opentelemetry-collector-contrib/extension/dynamicconfig

go 1.14

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.2
	github.com/grpc-ecosystem/grpc-gateway v1.14.6
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig v0.0.0-00010101000000-000000000000
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.8.0
	go.uber.org/zap v1.15.0
	google.golang.org/grpc v1.31.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig => ./

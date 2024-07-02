module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gelfreceiver

go 1.22.0

require (
	github.com/Graylog2/go-gelf v0.0.0-20170811154226-7ebf4f536d8f
	go.opentelemetry.io/collector/component v0.102.1
	go.opentelemetry.io/collector/consumer v0.102.1
	go.opentelemetry.io/collector/receiver v0.102.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.102.0
)

require (
	github.com/go-viper/mapstructure/v2 v2.0.0-alpha.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.102.1 // indirect
	go.opentelemetry.io/collector/confmap v0.102.1 // indirect
	go.opentelemetry.io/collector/pdata v1.9.0 // indirect
	go.opentelemetry.io/otel v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.27.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240520151616-dc85e6b867a5 // indirect
	google.golang.org/grpc v1.64.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
)


replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza
replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gelfreceiver/internal/metadata => ../../internal/metadata
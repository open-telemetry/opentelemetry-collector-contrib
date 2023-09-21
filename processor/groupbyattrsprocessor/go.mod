module github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor

go 1.20

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.85.0
	github.com/stretchr/testify v1.8.4
	go.opencensus.io v0.24.0
	go.opentelemetry.io/collector v0.85.1-0.20230919160920-512b9c3c8fec
	go.opentelemetry.io/collector/component v0.85.1-0.20230919160920-512b9c3c8fec
	go.opentelemetry.io/collector/confmap v0.85.1-0.20230919160920-512b9c3c8fec
	go.opentelemetry.io/collector/consumer v0.85.1-0.20230919160920-512b9c3c8fec
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0014.0.20230919160920-512b9c3c8fec
	go.opentelemetry.io/collector/processor v0.85.1-0.20230919160920-512b9c3c8fec
	go.uber.org/zap v1.26.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.85.1-0.20230919160920-512b9c3c8fec // indirect
	go.opentelemetry.io/collector/exporter v0.85.1-0.20230919160920-512b9c3c8fec // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0014.0.20230919160920-512b9c3c8fec // indirect
	go.opentelemetry.io/collector/receiver v0.85.1-0.20230919160920-512b9c3c8fec // indirect
	go.opentelemetry.io/otel v1.18.0 // indirect
	go.opentelemetry.io/otel/metric v1.18.0 // indirect
	go.opentelemetry.io/otel/sdk v1.18.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.41.0 // indirect
	go.opentelemetry.io/otel/trace v1.18.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230711160842-782d3b101e98 // indirect
	google.golang.org/grpc v1.58.1 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

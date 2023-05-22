module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver

go 1.19

require (
	github.com/influxdata/influxdb-client-go/v2 v2.12.3
	github.com/influxdata/influxdb-observability/common v0.3.14
	github.com/influxdata/influxdb-observability/influx2otel v0.3.14
	github.com/influxdata/influxdb1-client v0.0.0-20220302092344-a9ab5670611c
	github.com/influxdata/line-protocol/v2 v2.2.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.77.0
	github.com/stretchr/testify v1.8.2
	go.opentelemetry.io/collector v0.78.1
	go.opentelemetry.io/collector/component v0.78.1
	go.opentelemetry.io/collector/consumer v0.78.1
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012
	go.opentelemetry.io/collector/receiver v0.78.1
	go.uber.org/zap v1.24.0
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deepmap/oapi-codegen v1.8.2 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/frankban/quicktest v1.14.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/influxdata/line-protocol v0.0.0-20200327222509-2487e7298839 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.5 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.77.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.9.0 // indirect
	go.opentelemetry.io/collector/confmap v0.78.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012 // indirect
	go.opentelemetry.io/collector/semconv v0.78.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.41.1 // indirect
	go.opentelemetry.io/otel v1.15.1 // indirect
	go.opentelemetry.io/otel/metric v0.38.1 // indirect
	go.opentelemetry.io/otel/trace v1.15.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

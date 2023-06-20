module github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector

go 1.19

require (
	github.com/hashicorp/golang-lru v0.6.0
	github.com/lightstep/go-expohisto v1.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.80.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.80.0
	github.com/stretchr/testify v1.8.4
	github.com/tilinna/clock v1.1.0
	go.opentelemetry.io/collector/component v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/confmap v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/connector v0.0.0-20230615165320-df20186ee21c
	go.opentelemetry.io/collector/consumer v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012.0.20230619144035-25129b794d48
	go.opentelemetry.io/collector/semconv v0.79.1-0.20230620160303-6059751e64e0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	google.golang.org/grpc v1.56.0
)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230619144035-25129b794d48 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.2.1 // indirect
	golang.org/x/net v0.11.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

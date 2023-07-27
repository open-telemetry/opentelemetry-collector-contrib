module bitbucket.org/gisfederal/opentelemetry/exporter/kineticaexporter

go 1.20

require (
	bitbucket.org/gisfederal/gpudb-api-go v0.0.8
	go.opentelemetry.io/collector/component v0.76.1
	go.opentelemetry.io/collector/exporter v0.76.1
	go.uber.org/zap v1.24.0

)

require github.com/pmezard/go-difflib v1.0.0 // indirect

require (
	github.com/samber/lo v1.38.1
	golang.org/x/exp v0.0.0-20220303212507-bbda1eaf7a17 // indirect
)

require (
	github.com/go-resty/resty/v2 v2.7.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hamba/avro v1.8.0 // indirect
	github.com/logrusorgru/aurora v2.0.3+incompatible // indirect
	github.com/ztrue/tracerr v0.3.0 // indirect
	go.opentelemetry.io/collector v0.76.1
	go.opentelemetry.io/collector/confmap v0.76.1
	go.opentelemetry.io/collector/consumer v0.76.1 // indirect
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0011
	go.uber.org/multierr v1.11.0
)

require (
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.0
	github.com/influxdata/influxdb-observability/common v0.3.9
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/stretchr/testify v1.8.4
	github.com/wk8/go-ordered-map v1.0.0
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/featuregate v0.76.1 // indirect
	go.opentelemetry.io/collector/receiver v0.76.1 // indirect
	go.opentelemetry.io/collector/semconv v0.77.0
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.opentelemetry.io/otel/metric v0.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.14.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	google.golang.org/grpc v1.54.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

retract v0.65.0

module github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter

go 1.18

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.70.0
	github.com/tinylib/msgp v1.1.8
	go.opentelemetry.io/collector v0.70.0
	go.opentelemetry.io/collector/component v0.70.0
	go.opentelemetry.io/collector/consumer v0.70.0
	go.opentelemetry.io/collector/pdata v1.0.0-rc4
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	github.com/apache/thrift v0.17.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.15.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jaegertracing/jaeger v1.41.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.1.17 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.70.0 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.1 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/prometheus/statsd_exporter v0.22.7 // indirect
	github.com/rs/cors v1.8.3 // indirect
	github.com/shirou/gopsutil/v3 v3.22.12 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/cobra v1.6.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/testify v1.8.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/confmap v0.70.0 // indirect
	go.opentelemetry.io/collector/exporter/loggingexporter v0.70.0 // indirect
	go.opentelemetry.io/collector/exporter/otlpexporter v0.70.0 // indirect
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.70.0 // indirect
	go.opentelemetry.io/collector/extension/ballastextension v0.70.0 // indirect
	go.opentelemetry.io/collector/extension/zpagesextension v0.70.0 // indirect
	go.opentelemetry.io/collector/featuregate v0.70.0 // indirect
	go.opentelemetry.io/collector/processor/batchprocessor v0.70.0 // indirect
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.70.0 // indirect
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.70.0 // indirect
	go.opentelemetry.io/collector/semconv v0.70.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.37.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.37.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.12.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.37.0 // indirect
	go.opentelemetry.io/otel v1.11.2 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.34.0 // indirect
	go.opentelemetry.io/otel/metric v0.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.11.2 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.11.2 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	golang.org/x/text v0.6.0 // indirect
	gonum.org/v1/gonum v0.12.0 // indirect
	google.golang.org/genproto v0.0.0-20221227171554-f9683d7f8bef // indirect
	google.golang.org/grpc v1.52.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

module github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter

go 1.19

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	github.com/tinylib/msgp v1.1.8
	go.opentelemetry.io/collector/component v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/config/confighttp v0.0.0-20230619144035-25129b794d48
	go.opentelemetry.io/collector/consumer v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/exporter v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012.0.20230619144035-25129b794d48
)

require (
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.16.6 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/rs/cors v1.9.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.79.1-0.20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/config/configopaque v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/config/configtls v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/config/internal v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/confmap v0.79.1-0.20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/extension v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230619144035-25129b794d48 // indirect
	go.opentelemetry.io/collector/processor v0.0.0-20230609193203-89d1060c7606 // indirect
	go.opentelemetry.io/collector/receiver v0.79.1-0.20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.42.0 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.2.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/net v0.11.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.56.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
)

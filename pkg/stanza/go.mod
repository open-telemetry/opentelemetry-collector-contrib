module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza

go 1.19

require (
	github.com/antonmedv/expr v1.12.5
	github.com/bmatcuk/doublestar/v4 v4.6.0
	github.com/cespare/xxhash/v2 v2.2.0
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20210608084020-ac565dc76ba6
	github.com/jpillora/backoff v1.0.0
	github.com/json-iterator/go v1.1.12
	github.com/observiq/ctimefmt v1.0.0
	github.com/observiq/nanojack v0.0.0-20201106172433-343928847ebc
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage v0.77.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.77.0
	github.com/stretchr/testify v1.8.2
	go.opentelemetry.io/collector v0.77.0
	go.opentelemetry.io/collector/component v0.77.0
	go.opentelemetry.io/collector/confmap v0.77.0
	go.opentelemetry.io/collector/consumer v0.77.0
	go.opentelemetry.io/collector/featuregate v0.77.0
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012
	go.opentelemetry.io/collector/receiver v0.77.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	golang.org/x/sys v0.8.0
	golang.org/x/text v0.9.0
	gonum.org/v1/gonum v0.13.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20181214104525-299bdde78165 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector/exporter v0.77.0 // indirect
	go.opentelemetry.io/otel v1.15.1 // indirect
	go.opentelemetry.io/otel/metric v0.38.1 // indirect
	go.opentelemetry.io/otel/trace v1.15.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	google.golang.org/genproto v0.0.0-20230306155012-7f2fa6fef1f4 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/googleapis/gnostic v0.5.6 => github.com/googleapis/gnostic v0.5.5

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../pdatatest

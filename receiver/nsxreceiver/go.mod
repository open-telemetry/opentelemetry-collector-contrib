module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxreceiver

go 1.17

require (
	go.opentelemetry.io/collector v0.50.1-0.20220429151328-041f39835df7
	go.opentelemetry.io/collector/pdata v0.50.1-0.20220429151328-041f39835df7
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/knadh/koanf v1.4.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/vmware/go-vmware-nsxt v0.0.0-20220328155605-f49a14c1ef5f
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/collector/model v0.49.0
	go.opentelemetry.io/otel v1.7.0 // indirect
	go.opentelemetry.io/otel/metric v0.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.7.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)

require (
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.0.0-00010101000000-000000000000
	github.com/rs/cors v1.8.2 // indirect
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.32.0 // indirect
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/grpc v1.46.0 // indirect
)

require (
	github.com/antonmedv/expr v1.9.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20210608084020-ac565dc76ba6 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/observiq/ctimefmt v1.0.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza v0.50.0 // indirect
	github.com/open-telemetry/opentelemetry-log-collection v0.29.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad // indirect
	gonum.org/v1/gonum v0.11.0 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest => ../../internal/scrapertest

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver => ../syslogreceiver

replace go.opentelemetry.io/collector/semconv => go.opentelemetry.io/collector/semconv v0.0.0-20220422001137-87ab5de64ce4

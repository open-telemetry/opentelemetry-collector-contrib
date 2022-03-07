module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.17

require (
	github.com/DataDog/agent-payload/v5 v5.0.16
	github.com/DataDog/datadog-agent/pkg/quantile v0.33.1
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	github.com/aws/aws-sdk-go v1.43.8
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/gogo/protobuf v1.3.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.46.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.46.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.46.0
	go.opentelemetry.io/collector/model v0.46.0
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0
	gopkg.in/DataDog/dd-trace-go.v1 v1.36.2
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0
)

require (
	github.com/DataDog/datadog-agent/pkg/util/log v0.0.0-20201009092105-58e18918b2db // indirect
	github.com/DataDog/datadog-go v4.8.2+incompatible // indirect
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/knadh/koanf v1.4.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/shirou/gopsutil v2.20.9+incompatible // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/tinylib/msgp v1.1.2 // indirect
	github.com/zorkian/go-datadog-api v2.30.0+incompatible // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/otel v1.4.1 // indirect
	go.opentelemetry.io/otel/metric v0.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.4.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9 // indirect
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

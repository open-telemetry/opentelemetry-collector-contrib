module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver

go 1.22.0

require (
	github.com/Khan/genqlient v0.7.0
	github.com/google/go-cmp v0.6.0
	github.com/google/go-github/v66 v66.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.113.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.113.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.113.0
	go.opentelemetry.io/collector/config/confighttp v0.113.0
	go.opentelemetry.io/collector/confmap v1.19.0
	go.opentelemetry.io/collector/consumer v0.113.0
	go.opentelemetry.io/collector/consumer/consumertest v0.113.0
	go.opentelemetry.io/collector/filter v0.113.0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.113.0
	go.opentelemetry.io/collector/pdata v1.19.0
	go.opentelemetry.io/collector/pipeline v0.113.0
	go.opentelemetry.io/collector/receiver v0.113.0
	go.opentelemetry.io/collector/receiver/receivertest v0.113.0
	go.opentelemetry.io/collector/semconv v0.113.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/purego v0.8.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20240408141607-282e7b5d6b74 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.113.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.60.1 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v4 v4.24.10 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/vektah/gqlparser/v2 v2.5.16 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/collector/client v1.19.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.113.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.113.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.19.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.19.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.113.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.19.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.113.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.19.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.19.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.19.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.19.0 // indirect
	go.opentelemetry.io/collector/connector v0.113.0 // indirect
	go.opentelemetry.io/collector/connector/connectorprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.113.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.113.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/exporter v0.113.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.113.0 // indirect
	go.opentelemetry.io/collector/extension v0.113.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.113.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.113.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.19.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.113.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.113.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.113.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.113.0 // indirect
	go.opentelemetry.io/collector/pipeline/pipelineprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/processor v0.113.0 // indirect
	go.opentelemetry.io/collector/processor/processorprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.113.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/service v0.113.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.6.0 // indirect
	go.opentelemetry.io/contrib/config v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.31.0 // indirect
	go.opentelemetry.io/otel v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.7.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.53.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.7.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.31.0 // indirect
	go.opentelemetry.io/otel/log v0.7.0 // indirect
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.7.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.31.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

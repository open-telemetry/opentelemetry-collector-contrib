module github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension

go 1.23.0

require (
	github.com/DataDog/datadog-agent/pkg/serializer v0.64.3
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.125.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.31.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/component/componentstatus v0.125.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/component/componenttest v0.125.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/config/confighttp v0.125.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/confmap v1.31.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/extension v1.31.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.125.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/extension/extensiontest v0.125.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/otelcol v0.125.1-0.20250501194116-727ae96d6214
	go.opentelemetry.io/collector/service v0.125.1-0.20250501194116-727ae96d6214
)

require (
	github.com/DataDog/datadog-agent/pkg/proto v0.66.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.64.3 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.64.3 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.64.3 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.64.3 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.27.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.27.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.27.0 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.21.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v4 v4.25.3 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/tinylib/msgp v1.2.5 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configauth v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/confignet v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configretry v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/config/configtls v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/connector v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/consumer v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/exporter v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/featuregate v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/pdata v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/pipeline v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/processor v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/receiver v1.31.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/semconv v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.125.1-0.20250501194116-727ae96d6214 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.15.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.57.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250210185358-939b2ce775ac // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250224174004-546df14abb99 // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog => ../../pkg/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../internal/datadog

module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension

go 1.23.0

require (
	github.com/google/uuid v1.6.0
	github.com/oklog/ulid/v2 v2.1.0
	github.com/open-telemetry/opamp-go v0.19.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages v0.121.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status v0.121.0
	github.com/shirou/gopsutil/v4 v4.25.2
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.27.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/component/componentstatus v0.121.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/component/componenttest v0.121.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/config/configopaque v1.27.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/config/configtls v1.27.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/confmap v1.27.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/extension v1.27.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/extension/extensionauth v0.121.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.121.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/extension/extensiontest v0.121.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/semconv v0.121.1-0.20250313100724-0885401136ff
	go.opentelemetry.io/collector/service v0.121.1-0.20250313100724-0885401136ff
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	google.golang.org/grpc v1.71.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.21.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/connector v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/consumer v1.27.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/exporter v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/featuregate v1.27.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/processor v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/processor/processortest v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/receiver v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.15.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
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
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
)

require (
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/collector/pdata v1.27.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/collector/pipeline v0.121.1-0.20250313100724-0885401136ff // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.5 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages => ../opampcustommessages

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status => ../../pkg/status

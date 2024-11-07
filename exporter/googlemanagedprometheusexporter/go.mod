module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter

go 1.22.7

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.49.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.49.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.112.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.112.0
	go.opentelemetry.io/collector/confmap v1.18.0
	go.opentelemetry.io/collector/consumer v0.112.0
	go.opentelemetry.io/collector/exporter v0.112.0
	go.opentelemetry.io/collector/exporter/exportertest v0.112.0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.112.0
	go.opentelemetry.io/collector/pdata v1.18.0
	go.uber.org/goleak v1.3.0
)

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.10.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.5 // indirect
	cloud.google.com/go/compute/metadata v0.5.2 // indirect
	cloud.google.com/go/logging v1.12.0 // indirect
	cloud.google.com/go/longrunning v0.6.2 // indirect
	cloud.google.com/go/monitoring v1.21.2 // indirect
	cloud.google.com/go/trace v1.11.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.25.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.49.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.60.1 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/shirou/gopsutil/v4 v4.24.9 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.1.7 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.112.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.112.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.18.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.112.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.18.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.18.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.18.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.18.0 // indirect
	go.opentelemetry.io/collector/connector v0.112.0 // indirect
	go.opentelemetry.io/collector/connector/connectorprofiles v0.112.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.112.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.112.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.112.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.112.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterprofiles v0.112.0 // indirect
	go.opentelemetry.io/collector/extension v0.112.0 // indirect
	go.opentelemetry.io/collector/extension/experimental/storage v0.112.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.112.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.18.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.112.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.112.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.112.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.112.0 // indirect
	go.opentelemetry.io/collector/pipeline/pipelineprofiles v0.112.0 // indirect
	go.opentelemetry.io/collector/processor v0.112.0 // indirect
	go.opentelemetry.io/collector/processor/processorprofiles v0.112.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.112.0 // indirect
	go.opentelemetry.io/collector/receiver v0.112.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.112.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.112.0 // indirect
	go.opentelemetry.io/collector/semconv v0.112.0 // indirect
	go.opentelemetry.io/collector/service v0.112.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.6.0 // indirect
	go.opentelemetry.io/contrib/config v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.56.0 // indirect
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
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/oauth2 v0.23.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/api v0.204.0 // indirect
	google.golang.org/genproto v0.0.0-20241021214115-324edc3d5d38 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20241021214115-324edc3d5d38 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241021214115-324edc3d5d38 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/grpc/stats/opentelemetry v0.0.0-20241028142157-ada6787961b3 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.88.0
	v0.87.0
	v0.86.0
	v0.85.0
	v0.84.0
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace go.opentelemetry.io/collector/pdata => /Users/danoshin/Projects/opentelemetry-collector/pdata

replace go.opentelemetry.io/collector/exporter/debugexporter => /Users/danoshin/Projects/opentelemetry-collector/exporter/debugexporter

replace go.opentelemetry.io/collector/exporter => /Users/danoshin/Projects/opentelemetry-collector/exporter

replace go.opentelemetry.io/collector/consumer => /Users/danoshin/Projects/opentelemetry-collector/consumer

replace go.opentelemetry.io/collector/exporter/exporterprofiles => /Users/danoshin/Projects/opentelemetry-collector/exporter/exporterprofiles

replace go.opentelemetry.io/collector/receiver => /Users/danoshin/Projects/opentelemetry-collector/receiver

replace go.opentelemetry.io/collector/receiver/receiverprofiles => /Users/danoshin/Projects/opentelemetry-collector/receiver/receiverprofiles

replace go.opentelemetry.io/collector/receiver/otlpreceiver => /Users/danoshin/Projects/opentelemetry-collector/receiver/otlpreceiver

replace go.opentelemetry.io/collector/service => /Users/danoshin/Projects/opentelemetry-collector/service

replace go.opentelemetry.io/collector/connector/connectorprofiles => /Users/danoshin/Projects/opentelemetry-collector/connector/connectorprofiles

replace go.opentelemetry.io/collector/connector => /Users/danoshin/Projects/opentelemetry-collector/connector

replace go.opentelemetry.io/collector/processor => /Users/danoshin/Projects/opentelemetry-collector/processor

replace go.opentelemetry.io/collector/processor/processorprofiles => /Users/danoshin/Projects/opentelemetry-collector/processor/processorprofiles

replace go.opentelemetry.io/collector/processor/memorylimiterprocessor => /Users/danoshin/Projects/opentelemetry-collector/processor/memorylimiterprocessor

replace go.opentelemetry.io/collector/component => /Users/danoshin/Projects/opentelemetry-collector/component

replace go.opentelemetry.io/collector/component/componentstatus => /Users/danoshin/Projects/opentelemetry-collector/component/componentstatus

replace go.opentelemetry.io/collector/component/componentprofiles => /Users/danoshin/Projects/opentelemetry-collector/component/componentprofiles

replace go.opentelemetry.io/collector/receiver/receivertest => /Users/danoshin/Projects/opentelemetry-collector/receiver/receivertest

replace go.opentelemetry.io/collector/internal/sharedcomponent => /Users/danoshin/Projects/opentelemetry-collector/internal/sharedcomponent

replace go.opentelemetry.io/collector/pipeline => /Users/danoshin/Projects/opentelemetry-collector/pipeline

replace go.opentelemetry.io/collector/internal/fanoutconsumer => /Users/danoshin/Projects/opentelemetry-collector/internal/fanoutconsumer

replace go.opentelemetry.io/collector/consumer/consumererror => /Users/danoshin/Projects/opentelemetry-collector/consumer/consumererror

replace go.opentelemetry.io/collector/connector/forwardconnector => /Users/danoshin/Projects/opentelemetry-collector/connector/forwardconnector

replace go.opentelemetry.io/collector/pipeline/profiles => /Users/danoshin/Projects/opentelemetry-collector/pipeline/profiles

replace go.opentelemetry.io/collector/otelcol => /Users/danoshin/Projects/opentelemetry-collector/otelcol

replace go.opentelemetry.io/collector/config/configauth => /Users/danoshin/Projects/opentelemetry-collector/config/configauth

replace go.opentelemetry.io/collector/config/configgrpc => /Users/danoshin/Projects/opentelemetry-collector/config/configgrpc

replace go.opentelemetry.io/collector/config/confighttp => /Users/danoshin/Projects/opentelemetry-collector/config/confighttp

replace go.opentelemetry.io/collector/config/configtls => /Users/danoshin/Projects/opentelemetry-collector/config/configtls

replace go.opentelemetry.io/collector/config/configcompression => /Users/danoshin/Projects/opentelemetry-collector/config/configcompression

replace go.opentelemetry.io/collector/config/confignet => /Users/danoshin/Projects/opentelemetry-collector/config/confignet

replace go.opentelemetry.io/collector/config/configopaque => /Users/danoshin/Projects/opentelemetry-collector/config/configopaque

replace go.opentelemetry.io/collector/config/configretry => /Users/danoshin/Projects/opentelemetry-collector/config/configretry

replace go.opentelemetry.io/collector/config/configtelemetry => /Users/danoshin/Projects/opentelemetry-collector/config/configtelemetry

replace go.opentelemetry.io/collector/config/internal => /Users/danoshin/Projects/opentelemetry-collector/config/internal

replace go.opentelemetry.io/collector/confmap => /Users/danoshin/Projects/opentelemetry-collector/confmap

replace go.opentelemetry.io/collector/confmap/provider/envprovider => /Users/danoshin/Projects/opentelemetry-collector/confmap/provider/envprovider

replace go.opentelemetry.io/collector/confmap/provider/fileprovider => /Users/danoshin/Projects/opentelemetry-collector/confmap/provider/fileprovider

replace go.opentelemetry.io/collector/confmap/provider/httpprovider => /Users/danoshin/Projects/opentelemetry-collector/confmap/provider/httpprovider

replace go.opentelemetry.io/collector/confmap/provider/httpsprovider => /Users/danoshin/Projects/opentelemetry-collector/confmap/provider/httpsprovider

replace go.opentelemetry.io/collector/confmap/provider/yamlprovider => /Users/danoshin/Projects/opentelemetry-collector/confmap/provider/yamlprovider

replace go.opentelemetry.io/collector/consumer/consumerprofiles => /Users/danoshin/Projects/opentelemetry-collector/consumer/consumerprofiles

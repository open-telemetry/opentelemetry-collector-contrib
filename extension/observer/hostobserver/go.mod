module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver

go 1.22.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.112.0
	github.com/shirou/gopsutil/v4 v4.24.9
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.112.0
	go.opentelemetry.io/collector/confmap v1.18.0
	go.opentelemetry.io/collector/extension v0.112.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.112.0 // indirect
	go.opentelemetry.io/collector/pdata v1.18.0 // indirect
	go.opentelemetry.io/otel v1.31.0 // indirect
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.31.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

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

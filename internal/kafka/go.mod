module github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka

go 1.22.0

require (
	github.com/IBM/sarama v1.43.3
	github.com/aws/aws-sdk-go v1.55.5
	github.com/stretchr/testify v1.9.0
	github.com/xdg-go/scram v1.1.2
	go.opentelemetry.io/collector/config/configtls v1.18.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.18.0 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.23.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
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

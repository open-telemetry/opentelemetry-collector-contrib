module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter

go 1.23.0

require (
	github.com/microsoft/ApplicationInsights-Go v0.4.4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.122.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.122.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/component/componenttest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/config/configopaque v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/confmap v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/confmap/xconfmap v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer/consumererror v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/exporter v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/exporter/exportertest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/pdata v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/semconv v0.122.2-0.20250319144947-41a9ea7f7402
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.37.0
)

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/featuregate v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pipeline v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

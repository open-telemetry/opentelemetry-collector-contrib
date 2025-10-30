module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver

go 1.24.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.138.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.138.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters v0.138.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.44.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/component/componenttest v0.138.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/confmap v1.44.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/confmap/xconfmap v0.138.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/consumer v1.44.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/consumer/consumertest v0.138.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/pdata v1.44.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/pipeline v1.44.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/receiver v1.44.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/receiver/receivertest v0.138.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/scraper v0.138.1-0.20251028105454-507664fb82dc
	go.opentelemetry.io/collector/scraper/scraperhelper v0.138.1-0.20251028105454-507664fb82dc
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.138.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.138.1-0.20251028105454-507664fb82dc // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.138.1-0.20251028105454-507664fb82dc // indirect
	go.opentelemetry.io/collector/featuregate v1.44.1-0.20251028105454-507664fb82dc // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.138.1-0.20251028105454-507664fb82dc // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.138.1-0.20251028105454-507664fb82dc // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.138.1-0.20251028105454-507664fb82dc // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/grpc v1.76.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters => ../../pkg/winperfcounters

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

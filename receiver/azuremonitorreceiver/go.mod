module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver

go 1.24.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.20.0
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1
	github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics v1.3.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor v0.11.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v3 v3.0.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions v1.3.0
	github.com/google/go-cmp v0.7.0
	github.com/huandu/go-clone v1.7.3
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.142.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.142.0
	github.com/orcaman/concurrent-map/v2 v2.0.1
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/component/componenttest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/confmap v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer/consumertest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/pdata v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/receiver v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/receiver/receivertest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/scraper v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/scraper/scraperhelper v0.142.1-0.20251231114627-ad49dc64648d
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.1
)

require go.opentelemetry.io/collector/confmap/xconfmap v0.142.1-0.20251231114627-ad49dc64648d

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.142.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/featuregate v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pipeline v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/otel v1.39.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

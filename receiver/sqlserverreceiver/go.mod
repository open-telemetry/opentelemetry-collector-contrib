module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver

go 1.23.0

require (
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.64.3
	github.com/google/go-cmp v0.7.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/microsoft/go-mssqldb v1.8.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery v0.125.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.125.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.125.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters v0.125.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.31.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/component/componenttest v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/config/configopaque v1.31.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/confmap v1.31.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/confmap/xconfmap v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/consumer v1.31.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/consumer/consumertest v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/filter v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/pdata v1.31.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/receiver v1.31.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/receiver/receivertest v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/scraper v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/collector/scraper/scraperhelper v0.125.1-0.20250509035855-4d929a9d6a7e
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/DataDog/datadog-go/v5 v5.6.0 // indirect
	github.com/DataDog/go-sqllexer v0.1.3 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.125.0 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/featuregate v1.31.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/pipeline v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.125.1-0.20250509035855-4d929a9d6a7e // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
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

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery => ../../internal/sqlquery

module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro

go 1.26.4

require (
	github.com/go-logfmt/logfmt v0.6.1
	github.com/grafana/faro/pkg/go v0.0.0-20261023195816-bd48199c6aee
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.164.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.164.1
	github.com/stretchr/testify v1.12.1
	github.com/wk8/go-ordered-map/v2 v2.1.8
	github.com/zeebo/xxh3 v1.1.0
	go.opentelemetry.io/collector/featuregate v1.70.1-0.20261029175530-9afdf701f333
	go.opentelemetry.io/collector/pdata v1.70.1-0.20261029175530-9afdf701f333
	go.opentelemetry.io/otel v1.47.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/oapi-codegen/runtime v1.7.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.164.1-0.20261029175530-9afdf701f333 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.164.1-0.20261029175530-9afdf701f333 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.48.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pdatatest

module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden

go 1.25.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.146.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/pdata v1.52.0
	go.opentelemetry.io/collector/pdata/pprofile v0.146.1
	go.uber.org/goleak v1.3.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.52.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../pdatautil

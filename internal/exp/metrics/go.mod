module github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics

go 1.24.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.137.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.137.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.137.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/pdata v1.43.1-0.20251013162618-a96eab114ea4
	go.opentelemetry.io/otel v1.38.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.43.1-0.20251013162618-a96eab114ea4 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.137.1-0.20251013162618-a96eab114ea4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/grpc v1.76.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../../pkg/pdatatest

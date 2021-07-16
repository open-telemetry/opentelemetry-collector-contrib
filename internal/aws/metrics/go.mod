module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/otel v0.20.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

replace go.opentelemetry.io/collector => go.opentelemetry.io/collector v0.29.1-0.20210716020257-4d8e3082465d

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210716020257-4d8e3082465d

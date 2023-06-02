module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics

go 1.19

require (
	github.com/stretchr/testify v1.8.3
	go.opentelemetry.io/otel v1.16.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

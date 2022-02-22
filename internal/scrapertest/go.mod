module github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest

go 1.17

require (
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector/model v0.43.3-0.20220201020338-caead4c4b0e2
	go.uber.org/multierr v1.7.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper => ../../receiver/scraperhelper

module github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest

go 1.17

require (
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector/model v0.47.1-0.20220322153344-4c3b824aca1d
	go.uber.org/multierr v1.8.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper => ../../receiver/scraperhelper

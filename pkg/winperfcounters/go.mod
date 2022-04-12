module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters

go 1.17

require (
	github.com/stretchr/testify v1.7.1
	go.uber.org/multierr v1.8.0
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.6.2 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest => ../../internal/scrapertest

replace go.opentelemetry.io/collector/pdata => go.opentelemetry.io/collector/pdata v0.0.0-20220412005140-8eb68f40028d

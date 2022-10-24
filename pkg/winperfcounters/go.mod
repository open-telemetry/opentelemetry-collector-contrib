module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters

go 1.18

require (
	github.com/stretchr/testify v1.8.1
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.6.2 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest => ../../internal/scrapertest

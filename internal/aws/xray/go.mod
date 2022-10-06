module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray

go 1.18

require (
	github.com/aws/aws-sdk-go v1.44.110
	github.com/stretchr/testify v1.8.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.opentelemetry.io/collector => github.com/dmitryax/opentelemetry-collector v0.49.1-0.20221006222824-5b0277b73df3

replace go.opentelemetry.io/collector/pdata => github.com/dmitryax/opentelemetry-collector/pdata v0.0.0-20221006222824-5b0277b73df3

module github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil

go 1.23.0

require github.com/stretchr/testify v1.10.0

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-20250424234037-ac7c0f2f4cd8

replace go.opentelemetry.io/collector/config/configmiddleware v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/config/configmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8

replace go.opentelemetry.io/collector/extension/extensionmiddleware v1.30.0 => go.opentelemetry.io/collector/extension/extensionmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8

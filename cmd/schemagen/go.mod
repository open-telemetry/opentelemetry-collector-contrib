module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen

go 1.25.0

require github.com/open-telemetry/opentelemetry-collector-contrib/pkg/schemagen v0.0.0

require (
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	golang.org/x/mod v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/tools v0.45.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/schemagen => ../../pkg/schemagen

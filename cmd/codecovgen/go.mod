module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/codecovgen

go 1.24.0

require (
	github.com/bmatcuk/doublestar/v4 v4.9.2
	go.yaml.in/yaml/v3 v3.0.4
	golang.org/x/mod v0.28.0
)

require (
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

// Can be removed after 0.144.0 release
replace go.opentelemetry.io/collector/internal/componentalias => go.opentelemetry.io/collector/internal/componentalias v0.0.0-20260115162016-5e41fb551263

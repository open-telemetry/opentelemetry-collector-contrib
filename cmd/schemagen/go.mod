module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/schemagen

go 1.24.0

require (
	github.com/stretchr/testify v1.11.1
	golang.org/x/tools v0.39.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
)

// Can be removed after 0.144.0 release
replace go.opentelemetry.io/collector/internal/componentalias => go.opentelemetry.io/collector/internal/componentalias v0.0.0-20260109195331-fbd5d3f9faae

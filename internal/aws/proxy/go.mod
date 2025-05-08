module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.23.0

require (
	github.com/aws/aws-sdk-go v1.55.7
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.125.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/config/confignet v1.31.1-0.20250508034258-ac520a5c14cc
	go.opentelemetry.io/collector/config/configtls v1.31.1-0.20250508034258-ac520a5c14cc
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/google/go-tpm v0.9.4 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.31.1-0.20250508034258-ac520a5c14cc // indirect
	go.opentelemetry.io/collector/featuregate v1.31.1-0.20250508034258-ac520a5c14cc // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

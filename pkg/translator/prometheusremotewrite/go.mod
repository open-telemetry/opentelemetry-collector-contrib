module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite

go 1.17

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.48.0
	github.com/prometheus/common v0.33.0
	github.com/prometheus/prometheus v1.8.2-0.20220117154355-4855a0c067e2
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector v0.48.1-0.20220412005140-8eb68f40028d
	go.opentelemetry.io/collector/model v0.48.1-0.20220412005140-8eb68f40028d
	go.opentelemetry.io/collector/pdata v0.0.0-00010101000000-000000000000
	go.uber.org/multierr v1.8.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace go.opentelemetry.io/collector/pdata => go.opentelemetry.io/collector/pdata v0.0.0-20220412005140-8eb68f40028d

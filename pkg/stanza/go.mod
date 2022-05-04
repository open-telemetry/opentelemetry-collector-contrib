module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza

go 1.17

require (
	github.com/antonmedv/expr v1.9.0
	github.com/json-iterator/go v1.1.12
	github.com/mitchellh/mapstructure v1.5.0
	github.com/observiq/ctimefmt v1.0.0
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector v0.50.1-0.20220429151328-041f39835df7
	go.uber.org/zap v1.21.0
	golang.org/x/exp v0.0.0-20200331195152-e8c3332aa8e5 // indirect
	golang.org/x/text v0.3.7
	gonum.org/v1/gonum v0.11.0
	gopkg.in/yaml.v2 v2.4.0
)

require go.uber.org/multierr v1.8.0

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/googleapis/gnostic v0.5.6 => github.com/googleapis/gnostic v0.5.5

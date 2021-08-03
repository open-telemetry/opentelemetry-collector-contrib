module github.com/open-telemetry/opentelemetry-log-collection

go 1.15

require (
	github.com/antonmedv/expr v1.8.9
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/jpillora/backoff v1.0.0
	github.com/json-iterator/go v1.1.11
	github.com/mitchellh/mapstructure v1.4.1
	github.com/observiq/ctimefmt v1.0.0
	github.com/observiq/go-syslog/v3 v3.0.2
	github.com/observiq/nanojack v0.0.0-20201106172433-343928847ebc
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.0
	go.uber.org/zap v1.18.1
	golang.org/x/exp v0.0.0-20200331195152-e8c3332aa8e5 // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1
	golang.org/x/text v0.3.6
	gonum.org/v1/gonum v0.9.3
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
)

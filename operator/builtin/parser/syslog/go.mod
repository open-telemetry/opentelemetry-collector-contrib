module github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/parser/syslog

go 1.14

require (
	github.com/observiq/go-syslog/v3 v3.0.2
	github.com/opentelemetry/opentelemetry-log-collection v0.13.12
	github.com/stretchr/testify v1.6.1
)

replace github.com/opentelemetry/opentelemetry-log-collection => ../../../../

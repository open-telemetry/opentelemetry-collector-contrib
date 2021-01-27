module github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/input/windows

go 1.14

require (
	github.com/opentelemetry/opentelemetry-log-collection v0.13.12
	github.com/stretchr/testify v1.6.1
	golang.org/x/sys v0.0.0-20201015000850-e3ed0017c211
	golang.org/x/text v0.3.3
)

replace github.com/opentelemetry/opentelemetry-log-collection => ../../../../

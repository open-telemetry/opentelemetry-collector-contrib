module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal

go 1.16

require (
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210630154722-7d0a0398174e

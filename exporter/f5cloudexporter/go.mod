module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter

go 1.16

require (
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210708142535-fdb85b3e868c
	go.uber.org/zap v1.18.1
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
	google.golang.org/api v0.50.0
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210708142535-fdb85b3e868c

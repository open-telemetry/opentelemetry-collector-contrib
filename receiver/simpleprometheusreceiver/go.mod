module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver

go 1.16

require (
	github.com/hashicorp/go-msgpack v0.5.5 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/prometheus/common v0.30.0
	github.com/prometheus/prometheus v1.8.2-0.20210621150501-ff58416a0b02
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210804191544-3cfe4f8c5d3e
	go.opentelemetry.io/collector/model v0.31.1-0.20210804191544-3cfe4f8c5d3e // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	k8s.io/client-go v0.21.3
)

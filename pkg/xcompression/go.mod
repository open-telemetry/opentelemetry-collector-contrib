module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression

go 1.26.0

require (
	github.com/klauspost/compress v1.20.4
	github.com/pierrec/lz4 v2.6.1+incompatible
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/config/configcompression v1.71.1-0.20261119222305-0c7a74cd0a4a
	go.uber.org/goleak v1.3.0
)

require (
	github.com/frankban/quicktest v1.14.6 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

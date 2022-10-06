module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen

go 1.18

require github.com/spf13/cobra v1.5.0

require (
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace go.opentelemetry.io/collector => github.com/dmitryax/opentelemetry-collector v0.49.1-0.20221006222824-5b0277b73df3

replace go.opentelemetry.io/collector/pdata => github.com/dmitryax/opentelemetry-collector/pdata v0.0.0-20221006222824-5b0277b73df3

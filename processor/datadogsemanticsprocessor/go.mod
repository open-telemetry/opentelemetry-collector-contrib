module github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor

go 1.25.0

require (
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.76.0-rc.4
	github.com/DataDog/datadog-agent/pkg/trace v0.76.0-rc.4
	github.com/DataDog/datadog-agent/pkg/trace/otel v0.76.0-rc.4
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/component/componentstatus v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/processor v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/processor/processorhelper v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/processor/processortest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/otel v1.40.0
)

require (
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/stats v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-go/v5 v5.8.2 // indirect
	github.com/DataDog/go-sqllexer v0.1.12 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/shirou/gopsutil/v4 v4.25.12 // indirect
	github.com/tinylib/msgp v1.6.3 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

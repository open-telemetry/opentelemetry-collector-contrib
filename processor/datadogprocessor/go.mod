module github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor

go 1.19

require (
	github.com/DataDog/datadog-agent/pkg/trace v0.47.0-devel
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.5.0
	github.com/DataDog/sketches-go v1.4.2
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/component v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/consumer v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/exporter v0.79.1-0.20230620160303-6059751e64e0
	go.opentelemetry.io/collector/extension v0.0.0-20230619144035-25129b794d48
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012.0.20230619144035-25129b794d48
	go.opentelemetry.io/collector/processor v0.0.0-20230619144035-25129b794d48
	go.uber.org/zap v1.24.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.46.0-rc.2 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.46.0-rc.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.46.0-rc.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.46.0-rc.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.46.0-rc.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.46.0-rc.2 // indirect
	github.com/DataDog/datadog-go/v5 v5.1.1 // indirect
	github.com/DataDog/go-tuf v0.3.0--fix-localmeta-fork // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.5.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/containerd/cgroups v1.0.4 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/outcaste-io/ristretto v0.2.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.5.0 // indirect
	github.com/shirou/gopsutil/v3 v3.23.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tinylib/msgp v1.1.8 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.0.0-20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/confmap v0.79.1-0.20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230619144035-25129b794d48 // indirect
	go.opentelemetry.io/collector/semconv v0.79.1-0.20230620160303-6059751e64e0 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230321023759-10a507213a29 // indirect
	golang.org/x/mod v0.10.0 // indirect
	golang.org/x/net v0.11.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.9.1 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.56.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// It appears that the v0.2.0 tag was modified.  Replacing with v0.2.1
replace github.com/outcaste-io/ristretto v0.2.0 => github.com/outcaste-io/ristretto v0.2.1

retract (
	v0.76.2
	v0.76.1
)

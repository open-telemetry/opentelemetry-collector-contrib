// Deprecated: Use Datadog connector instead.
module github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogprocessor

go 1.20

require (
	github.com/DataDog/datadog-agent/pkg/proto v0.52.0-devel
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.13.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.13.0
	github.com/DataDog/sketches-go v1.4.4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.92.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.92.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/component v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/confmap v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/consumer v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/exporter v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/extension v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/pdata v1.0.2-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/processor v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/otel/metric v1.22.0
	go.opentelemetry.io/otel/trace v1.22.0
	go.uber.org/zap v1.26.0
	google.golang.org/protobuf v1.32.0
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.50.2 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.50.2 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.50.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.50.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.50.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.50.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.50.2 // indirect
	github.com/DataDog/datadog-go/v5 v5.1.1 // indirect
	github.com/DataDog/go-sqllexer v0.0.8 // indirect
	github.com/DataDog/go-tuf v1.0.2-0.5.2 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.13.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/containerd/cgroups/v3 v3.0.2 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.3 // indirect
	github.com/outcaste-io/ristretto v0.2.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/prometheus/statsd_exporter v0.22.7 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.7.0 // indirect
	github.com/shirou/gopsutil/v3 v3.23.12 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tinylib/msgp v1.1.9 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.2-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/semconv v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.44.1-0.20231201153405-6027c1ae76f2 // indirect
	go.opentelemetry.io/otel/sdk v1.22.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.22.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20230711023510-fffb14384f22 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.4.0 // indirect
	golang.org/x/tools v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/grpc v1.60.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// It appears that the v0.2.0 tag was modified.  Replacing with v0.2.1
replace github.com/outcaste-io/ristretto v0.2.0 => github.com/outcaste-io/ristretto v0.2.1

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../internal/datadog

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver

go 1.23.0

require (
	code.cloudfoundry.org/garden v0.0.0-20241023020423-a21e43a17f84
	github.com/cloudfoundry/go-cfclient/v3 v3.0.0-alpha.9
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.120.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/component/componenttest v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/confmap v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/confmap/xconfmap v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/extension v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/extension/extensiontest v0.120.1-0.20250226024140-8099e51f9a77
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	code.cloudfoundry.org/lager/v3 v3.11.0 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/codegangsta/inject v0.0.0-20150114235600-33e0aa1cb7c0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-martini/martini v0.0.0-20170121215854-22fa46961aab // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/pprof v0.0.0-20241023014458-598669927662 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/martini-contrib/render v0.0.0-20150707142108-ec18f8345a11 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/onsi/ginkgo/v2 v2.20.2 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tedsuo/rata v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/pdata v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/oauth2 v0.24.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/tools v0.26.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241202173237-19429a94021a // indirect
	google.golang.org/grpc v1.70.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../

replace go.opentelemetry.io/collector/extension/extensionauth => go.opentelemetry.io/collector/extension/extensionauth v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/service/hostcapabilities => go.opentelemetry.io/collector/service/hostcapabilities v0.0.0-20250226024140-8099e51f9a77

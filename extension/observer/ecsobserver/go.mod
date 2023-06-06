module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver

go 1.19

require (
	github.com/aws/aws-sdk-go v1.44.274
	github.com/hashicorp/golang-lru v0.5.4
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector v0.78.3-0.20230605151302-371fc4517197
	go.opentelemetry.io/collector/component v0.78.3-0.20230605151302-371fc4517197
	go.opentelemetry.io/collector/confmap v0.78.3-0.20230605151302-371fc4517197
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.24.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230605162713-2fbdd031c2f5 // indirect
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012.0.20230605162713-2fbdd031c2f5 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

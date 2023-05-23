module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver

go 1.19

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.78.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.78.0
	github.com/stretchr/testify v1.8.3
	go.opentelemetry.io/collector v0.78.2
	go.opentelemetry.io/collector/component v0.78.2
	go.opentelemetry.io/collector/confmap v0.78.2
	go.uber.org/zap v1.24.0
	k8s.io/api v0.27.2
	k8s.io/apimachinery v0.27.2
	k8s.io/client-go v0.27.2
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012 // indirect
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0012 // indirect
	go.opentelemetry.io/otel v1.15.1 // indirect
	go.opentelemetry.io/otel/metric v0.38.1 // indirect
	go.opentelemetry.io/otel/trace v1.15.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.55.0 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../../internal/k8sconfig

// openshift removed all tags from their repo, use the pseudoversion from the release-3.9 branch HEAD
replace github.com/openshift/api v3.9.0+incompatible => github.com/openshift/api v0.0.0-20180801171038-322a19404e37

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

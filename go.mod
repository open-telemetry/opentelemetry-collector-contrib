module github.com/open-telemetry/opentelemetry-log-collection

go 1.17

require (
	github.com/antonmedv/expr v1.9.0
	github.com/bmatcuk/doublestar/v3 v3.0.0
	github.com/jpillora/backoff v1.0.0
	github.com/json-iterator/go v1.1.12
	github.com/mitchellh/mapstructure v1.4.3
	github.com/observiq/ctimefmt v1.0.0
	github.com/observiq/go-syslog/v3 v3.1.0
	github.com/observiq/nanojack v0.0.0-20201106172433-343928847ebc
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.46.0
	go.uber.org/zap v1.21.0
	golang.org/x/exp v0.0.0-20200331195152-e8c3332aa8e5 // indirect
	golang.org/x/sys v0.0.0-20220307203707-22a9840ba4d7
	golang.org/x/text v0.3.7
	gonum.org/v1/gonum v0.9.3
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.4
	k8s.io/apimachinery v0.23.4
	k8s.io/client-go v0.23.4
)

require go.uber.org/multierr v1.8.0

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.6 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/klog/v2 v2.40.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220124234850-424119656bbf // indirect
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect
	sigs.k8s.io/json v0.0.0-20211208200746-9f7c6b3444d2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/googleapis/gnostic v0.5.6 => github.com/googleapis/gnostic v0.5.5

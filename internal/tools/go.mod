module github.com/open-telemetry/opentelemetry-collector-contrib/internal/tools

go 1.16

require (
	github.com/client9/misspell v0.3.4
	github.com/golangci/golangci-lint v1.41.1
	github.com/google/addlicense v1.0.0
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen v0.0.0-00010101000000-000000000000
	github.com/pavius/impi v0.0.3
	github.com/prometheus/common v0.29.0 // indirect
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/tcnksm/ghr v0.14.0
	go.opentelemetry.io/build-tools/checkdoc v0.0.0-20210826185254-a20c5b1e2d7c
	go.opentelemetry.io/build-tools/issuegenerator v0.0.0-20210826185254-a20c5b1e2d7c
	go.uber.org/atomic v1.8.0 // indirect
	golang.org/x/tools v0.1.3
	google.golang.org/protobuf v1.27.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen => ../../cmd/mdatagen

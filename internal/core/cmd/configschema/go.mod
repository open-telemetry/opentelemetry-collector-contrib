module github.com/open-telemetry/opentelemetry-collector-contrib/internal/core/cmd/configschema

go 1.16

require (
	github.com/fatih/structtag v1.2.0
	github.com/google/uuid v1.3.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210826200354-479f46434f9a
	go.opentelemetry.io/collector/model v0.33.0
	golang.org/x/mod v0.4.2
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/core/cmd/configschema => ./

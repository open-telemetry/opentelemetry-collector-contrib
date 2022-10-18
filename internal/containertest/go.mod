module github.com/open-telemetry/opentelemetry-collector-contrib/internal/containertest

go 1.18

require (
	github.com/cenkalti/backoff/v4 v4.1.3
	github.com/docker/docker v20.10.19+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/stretchr/testify v1.8.0
)

require (
	github.com/Microsoft/go-winio v0.5.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	golang.org/x/time v0.0.0-20220210224613-90d013bbcef8 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.1.0 // indirect
)

// see https://github.com/distribution/distribution/issues/3590
exclude github.com/docker/distribution v2.8.0+incompatible

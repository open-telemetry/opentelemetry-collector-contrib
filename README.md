<p align="center">
  <a href="https://goreportcard.com/report/github.com/open-telemetry/opentelemetry-collector-contrib">
    <img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/open-telemetry/opentelemetry-collector-contrib?style=for-the-badge">
  </a>
  <a href="https://circleci.com/gh/open-telemetry/opentelemetry-collector-contrib">
    <img alt="Build Status" src="https://img.shields.io/circleci/build/github/open-telemetry/opentelemetry-collector-contrib?style=for-the-badge">
  </a>
  <a href="https://codecov.io/gh/open-telemetry/opentelemetry-collector-contrib/branch/master/">
    <img alt="Codecov Status" src="https://img.shields.io/codecov/c/github/open-telemetry/opentelemetry-collector-contrib?style=for-the-badge">
  </a>
  <a href="releases">
    <img alt="GitHub release (latest by date including pre-releases)" src="https://img.shields.io/github/v/release/open-telemetry/opentelemetry-collector-contrib?include_prereleases&style=for-the-badge">
  </a>
</p>

# OpenTelemetry Collector Contrib
This is a repository for OpenTelemetry Collector contributions that are not part of the
[core repository](https://github.com/open-telemetry/opentelemetry-collector) and
core distribution of the Collector. Typically, these contributions are vendor
specific receivers/exporters and/or components that are only
useful to a relatively small number of users. 

## Docker Images
Docker images for all releases are published at https://hub.docker.com/r/otel/opentelemetry-collector-contrib

## Contributing
If you would like to contribute please read [contributing guidelines](https://github.com/open-telemetry/opentelemetry-collector/blob/master/CONTRIBUTING.md)
before you begin your work.

## Adding New Components
Before you start please read the [contributing guidelines](https://github.com/open-telemetry/opentelemetry-collector/blob/master/CONTRIBUTING.md).

Any component (receiver, processor, exporter, or extension) needs to implement 
the interfaces defined on the [core repository](https://github.com/open-telemetry/opentelemetry-collector).
Familiarize yourself with the interface of the component that you want to write,
and use existing implementations as reference.

*NOTICE:* The Collector is in Alpha stage and as such the interfaces may undergo
breaking changes. Component creators must be available to update or review
their components when such changes happen, otherwise the component will be excluded
from the default builds.

- Create your component under the proper folder and use
Go standard package naming recommendations.
- Use a boiler-plate Makefile that just references the one at top level, 
ie.: `include ../../Makefile.Common` - this allows you to build your component
with required build configurations for the contrib repo while avoiding building
the full repo during development.
- Each component has its own go.mod file. This allows custom builds of the
collector to take a limited sets of dependencies - so run `go mod` commands as 
appropriate for your component.
- Implement the needed interface on your component by importing the appropriate
component from the core repo.  Follow the pattern of existing components regarding
config and factory source files and tests. 
- Implement your component as appropriate. Provide end-to-end tests (or mock 
backend/client as appropriate). Target is to get 80% or more of code coverage.
- Add a README.md on the root of your component describing its configuration 
and usage, likely referencing some of the yaml files used in the component tests.
We also suggest that the yaml files used in tests have comments for all available
configuration settings so users can copy and modify them as needed.
- Add a `replace` directive at the root `go.mod` file so your component is included
in the build of the contrib executable. 

### General Recommendations
Below are some recommendations that apply to typical components. These are not rigid
rules and there are exceptions to them, but, take care considering when you are 
not following them.

- Avoid introducing batching, retries or worker pools directly on receivers and
exporters. Typically, these are general cases that can be better handled via
processors (that also can be reused by other receivers and exporters).
- When implementing exporters try to leverage the exporter helpers from the core
repo, see [exporterhelper package](https://github.com/open-telemetry/opentelemetry-collector/tree/master/exporter/exporterhelper).
This will ensure that the exporter provides [zPages](https://opencensus.io/zpages/)
and a standard set of metrics.

### Questions?
Reach the Collector community on [gitter](https://gitter.im/open-telemetry/opentelemetry-service)
if you have further questions.

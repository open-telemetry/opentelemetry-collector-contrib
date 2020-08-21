---

<p align="center">
  <strong>
    <a href="https://opentelemetry.io/docs/collector/about/">Getting Started<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/community#agentcollector">Getting Involved<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://gitter.im/open-telemetry/opentelemetry-service">Getting In Touch<a/>
  </strong>
</p>

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
  <img alt="Beta" src="https://img.shields.io/badge/status-beta-informational?style=for-the-badge&logo=data:image/png;base64,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">
</p>

<p align="center">
  <strong>
    <a href="https://github.com/open-telemetry/opentelemetry-collector/blob/master/CONTRIBUTING.md">Contributing<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/opentelemetry-collector/blob/master/docs/vision.md">Vision<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/opentelemetry-collector/blob/master/docs/design.md">Design<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/opentelemetry-collector/blob/master/docs/monitoring.md">Monitoring<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/opentelemetry-collector/blob/master/docs/performance.md">Performance<a/>
    &nbsp;&nbsp;&bull;&nbsp;&nbsp;
    <a href="https://github.com/open-telemetry/opentelemetry-collector/blob/master/docs/roadmap.md">Roadmap<a/>
  </strong>
</p>

---

# OpenTelemetry Collector Contrib
This is a repository for OpenTelemetry Collector contributions that are not part of the
[core repository](https://github.com/open-telemetry/opentelemetry-collector) and
core distribution of the Collector. Typically, these contributions are vendor
specific receivers/exporters and/or components that are only
useful to a relatively small number of users.

## Adding New Components
Before you start please read the [contributing
guidelines](https://github.com/open-telemetry/opentelemetry-collector/blob/master/CONTRIBUTING.md).

Any component (receiver, processor, exporter, or extension) needs to implement
the interfaces defined on the [core
repository](https://github.com/open-telemetry/opentelemetry-collector).
Familiarize yourself with the interface of the component that you want to
write, and use existing implementations as reference.

*NOTICE:* The Collector is in Beta stage and as such the interfaces may
undergo breaking changes. Component creators must be available to update or
review their components when such changes happen, otherwise the component will
be excluded from the default builds.

Generally, maintenance of components is the responsibility of contributors who
authored them. If the original author or some other contributor does not
maintain the component it may be excluded from the default build. The component
**will** be excluded if it causes build problems, has failing tests or
otherwise causes problems to the rest of the repository and the rest of
contributors.

- Create your component under the proper folder and use Go standard package
  naming recommendations.
- Use a boiler-plate Makefile that just references the one at top level, ie.:
  `include ../../Makefile.Common` - this allows you to build your component
  with required build configurations for the contrib repo while avoiding
  building the full repo during development.
- Each component has its own go.mod file. This allows custom builds of the
  collector to take a limited sets of dependencies - so run `go mod` commands
  as appropriate for your component.
- Implement the needed interface on your component by importing the appropriate
  component from the core repo.  Follow the pattern of existing components
  regarding config and factory source files and tests. 
- Implement your component as appropriate. Provide end-to-end tests (or mock
  backend/client as appropriate). Target is to get 80% or more of code
  coverage.
- Add a README.md on the root of your component describing its configuration
  and usage, likely referencing some of the yaml files used in the component
  tests. We also suggest that the yaml files used in tests have comments for
  all available configuration settings so users can copy and modify them as
  needed.
- Add a `replace` directive at the root `go.mod` file so your component is
  included in the build of the contrib executable.

### General Recommendations
Below are some recommendations that apply to typical components. These are not
rigid rules and there are exceptions but in general try to follow them.

- Avoid introducing batching, retries or worker pools directly on receivers and
  exporters. Typically, these are general cases that can be better handled via
  processors (that also can be reused by other receivers and exporters).
- When implementing exporters try to leverage the exporter helpers from the
  core repo, see [exporterhelper
  package](https://github.com/open-telemetry/opentelemetry-collector/tree/master/exporter/exporterhelper).
  This will ensure that the exporter provides
  [zPages](https://opencensus.io/zpages/) and a standard set of metrics.

### Questions
Reach the Collector community on
[gitter](https://gitter.im/open-telemetry/opentelemetry-service) if you have
further questions.

### Community Roles

Triagers ([@open-telemetry/collector-contrib-triagers](https://github.com/orgs/open-telemetry/teams/collector-contrib-triagers))
- [Steve Flanders](https://github.com/flands), Splunk

Approvers ([@open-telemetry/collector-contrib-approvers](https://github.com/orgs/open-telemetry/teams/collector-contrib-approvers)):

- [Dmitrii Anoshin](https://github.com/dmitryax), Splunk
- [James Bebbington](https://github.com/james-bebbington), Google
- [Jay Camp](https://github.com/jrcamp), Splunk
- [Nail Islamov](https://github.com/nilebox), Google
- [Owais Lone](https://github.com/owais), Splunk

Maintainers ([@open-telemetry/collector-contrib-maintainer](https://github.com/orgs/open-telemetry/teams/collector-contrib-maintainer)):

- [Bogdan Drutu](https://github.com/BogdanDrutu), Splunk
- [Paulo Janotti](https://github.com/pjanotti), Splunk
- [Tigran Najaryan](https://github.com/tigrannajaryan), Splunk

Learn more about roles in the [community repository](https://github.com/open-telemetry/community/blob/master/community-membership.md).

### Component Reviewers

#### Exporters

| Exporter | Reviewer(s) |
| -------- | ----------- |
| alibabacloudlogserviceexporter | @shabicheng |
| awsxrayexporter | @kbrockhoff @anuraaga |
| azuremonitorexporter | @pcwiese |
| carbonexporter | @pjanotti |
| elasticexporter | @axw |
| honeycombexporter | @paulosman @lizthegrey |
| jaegerthrifthttpexporter | @jpkrohling @pavolloffay |
| kinesisexporter | @owais |
| lightstepexporter | @austinlparker @jmacd |
| newrelicexporter | @MrAlias |
| sapmexporter | @owais @dmitryax |
| sentryexporter | @AbhiPrasad |
| signalfxexporter | @pmcollins @asuresh4 |
| splunkhecexporter | @atoulme |
| stackdriverexporter | @nilebox @james-bebbington |

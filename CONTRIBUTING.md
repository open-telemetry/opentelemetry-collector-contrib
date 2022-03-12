# Contributing

If you would like to contribute please read OpenTelemetry Collector [contributing
guidelines](https://github.com/open-telemetry/opentelemetry-collector/blob/main/CONTRIBUTING.md) before you begin your
work.

## Pull-request title

The title for your pull-request should contain the component type and name in brackets, plus a short statement for your
change. For instance:

    [processor/tailsampling] fix AND policy

## Adding New Components

**Before** any code is written, [open an
issue](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/new?assignees=&labels=new+component&template=new_component.md&title=New%20component)
providing the following information:

* Who's the sponsor for your component. A sponsor is an approver who will be in charge of being the official reviewer of
  the code and become a code owner for the component. For vendor-specific components, it's good to have a volunteer
  sponsor. If you can't find one, we'll assign one in a round-robin fashion. For non-vendor specific components, having
  a sponsor means that your use case has been validated.
* Some information about your component, such as the reasoning behind it, use-cases, telemetry data types supported, and
  anything else you think is relevant for us to make a decision about accepting the component.
* The configuration options your component will accept. This will help us understand what it does and have an idea of
  how the implementation might look like.


Any component (receiver, processor, exporter, or extension) needs to implement the interfaces defined on the [core
repository](https://github.com/open-telemetry/opentelemetry-collector). Familiarize yourself with the interface of the
component that you want to write, and use existing implementations as reference.

*NOTICE:* The Collector is in Beta stage and as such the interfaces may undergo breaking changes. Component creators
must be available to update or review their components when such changes happen, otherwise the component will be
excluded from the default builds.

Generally, maintenance of components is the responsibility of contributors who authored them. If the original author or
some other contributor does not maintain the component it may be excluded from the default build. The component **will**
be excluded if it causes build problems, has failing tests or otherwise causes problems to the rest of the repository
and the rest of contributors.

- Create your component under the proper folder and use Go standard package naming recommendations.
- Use a boiler-plate Makefile that just references the one at top level, ie.: `include ../../Makefile.Common` - this
  allows you to build your component with required build configurations for the contrib repo while avoiding building the
  full repo during development.
- Each component has its own go.mod file. This allows custom builds of the collector to take a limited sets of
  dependencies - so run `go mod` commands as appropriate for your component.
- Implement the needed interface on your component by importing the appropriate component from the core repo. Follow the
  pattern of existing components regarding config and factory source files and tests.
- Implement your component as appropriate. Provide end-to-end tests (or mock backend/client as appropriate). Target is
  to get 80% or more of code coverage.
- Add a README.md on the root of your component describing its configuration and usage, likely referencing some of the
  yaml files used in the component tests. We also suggest that the yaml files used in tests have comments for all
  available configuration settings so users can copy and modify them as needed.
- Add a `replace` directive at the root `go.mod` file so your component is included in the build of the contrib
  executable.
- Add your component to `versions.yaml`.
- All components must be included in [`internal/components/`](./internal/components) and in the respective testing
  harnesses. To align with the test goal of the project, components must be testable within the framework defined within
  the folder. If a component can not be properly tested within the existing framework, it must increase the non testable
  components number with a comment within the PR explaining as to why it can not be tested.

## General Recommendations
Below are some recommendations that apply to typical components. These are not rigid rules and there are exceptions but
in general try to follow them.

- Avoid introducing batching, retries or worker pools directly on receivers and exporters. Typically, these are general
  cases that can be better handled via processors (that also can be reused by other receivers and exporters).
- When implementing exporters try to leverage the exporter helpers from the core repo, see [exporterhelper
  package](https://github.com/open-telemetry/opentelemetry-collector/tree/main/exporter/exporterhelper). This will
  ensure that the exporter provides [zPages](https://opencensus.io/zpages/) and a standard set of metrics.

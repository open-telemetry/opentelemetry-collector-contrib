# Adding new components

> [!NOTE]  
> The OpenTelemetry Collector has a pluggable architecture that allows you to build your own
> [distribution](https://opentelemetry.io/docs/collector/distributions/) with your own [custom
> components](https://opentelemetry.io/docs/collector/building/) using the [OpenTelemetry Collector
> Builder](https://opentelemetry.io/docs/collector/custom-collector/). You **don't need** to include
> your component in this repository to be able to use or distribute your component: you can just 
> host it in your own repository as a Go module and [add it to the OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/).

You may donate an existing component or propose a whole new one. If you are writing a new component from scratch, **before** any code is written, [open an
issue](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/new?assignees=&labels=Sponsor+Needed%2Cneeds+triage&projects=&template=new_component.yaml&title=New+component%3A+)
providing the following information:

* Who's the sponsor for your component. A sponsor is an approver or maintainer who will be the official reviewer of the code and a code owner
  for the component. You will need to find a sponsor for the component in order for it to be accepted.
* Some information about your component, such as the reasoning behind it, use-cases, telemetry data types supported, and
  anything else you think is relevant for us to make a decision about accepting the component.
* The configuration options your component will accept. This will give us a better understanding of what it does, and 
  how it may be implemented.

Components refer to connectors, exporters, extensions, processors, and receivers. The key criteria to implementing a component is to:

* Implement the [component.Component](https://pkg.go.dev/go.opentelemetry.io/collector/component#Component) interface
* Provide a configuration structure which defines the configuration of the component
* Provide the implementation which performs the component operation
* Have a `metadata.yaml` file and its generated code (using [mdatadgen](https://github.com/open-telemetry/opentelemetry-collector/blob/main/cmd/mdatagen/README.md)).

Familiarize yourself with the interface of the component that you want to write, and use existing implementations as a reference.
[Building a Trace Receiver](https://opentelemetry.io/docs/collector/trace-receiver/) tutorial provides a detailed example of building a component.

> [!IMPORTANT]  
> The Collector is in Beta stage and as such the interfaces may undergo breaking changes. Component creators
> must be available to update or review their components when such changes happen, otherwise the component will be
> excluded from the default builds.

Generally, maintenance of components is the responsibility of contributors who authored them. If the original author or
some other contributor does not maintain the component it may be excluded from the default build. The component **will**
be excluded if it causes build problems, has failing tests, or otherwise causes problems to the rest of the repository
and its contributors.

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
- Run `make crosslink` to update intra-repository dependencies. It will add a `replace` directive to `go.mod` file of every intra-repository dependant. This is necessary for your component to be included in the contrib executable.
- Add your component to `versions.yaml`.
- All components included in the distribution must be included in
  [`cmd/otelcontribcol/builder-config.yaml`](./cmd/otelcontribcol/builder-config.yaml)
  and in the respective testing harnesses. To align with the test goal of the
  project, components must be testable within the framework defined within the
  folder. If a component can not be properly tested within the existing
  framework, it must increase the non testable components number with a comment
  within the PR explaining as to why it can not be tested. **(Note: this does
  not automatically include any components in official release binaries. See
  [Releasing new components](#releasing-new-components).)**

- Create a `metadata.yaml` file with at minimum the required fields defined in [metadata-schema.yaml](https://github.com/open-telemetry/opentelemetry-collector/blob/main/cmd/mdatagen/metadata-schema.yaml).
Here is a minimal representation:
```
type: <name of your component, such as apache, http, haproxy, postgresql>

status:
  class: <class of component, one of cmd, connector, exporter, extension, processor or receiver>
  stability:
    development: [<pick the signals supported: logs, metrics, traces. For extension, use "extension">]
  codeowners:
    active: [<github account of the sponsor, such as alice>, <your GitHub account if you are already an OpenTelemetry member>]
```
- Run `make generate-gh-issue-templates` to add your component to the dropdown list in the issue templates.
- For README.md, you can start with the following:
```
# <Title of your component>
<!-- status autogenerated section -->
<!-- end autogenerated section -->
```
- Create a `doc.go` file with a generate pragma. For a `fooreceiver`, the file will look like:
```
// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// Package fooreceiver bars.
package fooreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fooreceiver"
```
- Type `make generate`. This will trigger the [metadata generator](https://github.com/open-telemetry/opentelemetry-collector/blob/main/cmd/mdatagen/README.md#using-the-metadata-generator) to generate the associated code/documentation.
- Type `make gencodeowners`. This will trigger the regeneration of the `.github/CODEOWNERS` file. 

When submitting a component to the community, consider breaking it down into separate PRs as follows:

* **First PR** should include the overall structure of the new component:
  * Readme, configuration, and factory implementation usually using the helper
    factory structs.
  * This PR is usually trivial to review, so the size limit does not apply to
    it.
  * The component should use [`In Development` Stability](https://github.com/open-telemetry/opentelemetry-collector#development) in its README.
  * Before submitting a PR, run the following commands from the root of the repository to ensure your new component is meeting the repo linting expectations:
    * `make checkdoc`
    * `make checkmetadata`
    * `make checkapi`
    * `make goporto`
    * `make crosslink`
    * `make gotidy`
    * `make genotelcontribcol`
    * `make genoteltestbedcol`
    * `make generate`
    * `make multimod-verify`
    * `make generate-gh-issue-templates`
    * `make gengithub`
    * `make addlicense`
* **Second PR** should include the concrete implementation of the component. If the
  size of this PR is larger than the recommended size consider splitting it in
  multiple PRs.
* **Last PR** should mark the new component as `Alpha` stability.
  * Update its `metadata.yaml` file.
    * Mark the stability as `alpha`
    * Add `contrib` to the list of distributions
  * Add it to the `cmd/otelcontribcol` binary by updating the `cmd/otelcontribcol/builder-config.yaml` file.
  * Please also run:
    - `make generate`
    - `make genotelcontribcol`
 
  * The component must be enabled only after sufficient testing and only when it meets [`Alpha` stability requirements](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha).
* Once your component has reached `Alpha` stability, you may also submit a PR to the [OpenTelemetry Collector Releases](https://github.com/open-telemetry/opentelemetry-collector-releases) repository to include your component in future releases of the OpenTelemetry Collector `contrib` distribution.
* Once a new component has been added to the executable:
  * Please add the component
    to the [OpenTelemetry.io registry](https://github.com/open-telemetry/opentelemetry.io#adding-a-project-to-the-opentelemetry-registry).

### Releasing New Components
After a component has been merged it must be added to the
[OpenTelemetry Collector Contrib's release manifest.yaml](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/distributions/otelcol-contrib/manifest.yaml)
to be included in the distributed otelcol-contrib binaries and docker images.

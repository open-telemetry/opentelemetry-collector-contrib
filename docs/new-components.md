# Donating new components

This page explains how to add your own components to an OpenTelemetry Collector and how to donate
them to the
[opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib)
repository.

The OpenTelemetry Collector has a pluggable architecture that allows you to build your own
[distribution](https://opentelemetry.io/docs/collector/distributions/) with your own [custom
components](https://opentelemetry.io/docs/collector/building/) using the [OpenTelemetry Collector
Builder](https://opentelemetry.io/docs/collector/custom-collector/). You **don't need** to include
your component in this repository to be able to use or distribute your component: you can just host
it in your own repository as a Go module and add it to the [OpenTelemetry
registry](https://opentelemetry.io/ecosystem/registry/).

To donate your component you need to start by hosting your component outside of this repository as a
first step. This gives you time to develop your component and gather community feedback.

## Hosting your component outside of the [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) repository

This section explains how to go from a component idea to publishing and distributing your component.

### Exploring available options

Before building a new component, check if existing components on the [OpenTelemetry
registry](https://opentelemetry.io/ecosystem/registry/?s=&component=all&language=collector&flag=all)
can be a good fit for your use case. You can ask on the CNCF Slack #otel-collector channel to
understand how you may leverage existing components for your use case.

### Building your component

Components refer to connectors, exporters, extensions, processors, and receivers. As a first step,
we require that you build your component outside of the [opentelemetry-collector-contrib
repository](https://github.com/open-telemetry/opentelemetry-collector-contrib). This is also the
fastest way to start using your component and to publish it for others to consume if you want to. 

A component is a Go module (library) built using the `go.opentelemetry.io/collector` set of
libraries. These libraries contain examples (e.g. see the example on the
[`go.opentelemetry.io/collector/exporter` module
documentation](https://pkg.go.dev/go.opentelemetry.io/collector/exporter)). The official
 documentation also has a section on how to [build various kinds of
components](https://opentelemetry.io/docs/collector/building/). You can also use existing
implementations on this repository as a reference. The key criteria to implement a component is to:

* Implement the
  [component.Component](https://pkg.go.dev/go.opentelemetry.io/collector/component#Component)
  interface
* Provide a configuration structure which defines the configuration of the component
* Provide the implementation which performs the component operation

### Using your component

To use your component you can use the [OpenTelemetry Collector
Builder](https://opentelemetry.io/docs/collector/custom-collector/). Even if you don't publish your
component, you may [specify a local folder using the `replaces` option of the
builder](https://github.com/open-telemetry/opentelemetry-collector/tree/main/cmd/builder#configuration)
to include it in your build.

### Publishing your component

If you want to publish your component for other people to try it out, you can do so for free using
Github or other git forges. To do so, you need to [publish your component as a Go
module](https://go.dev/doc/modules/publishing). You can publish multiple components from a single
repository by including the path to the component in the tag: for example, the [`filelog`
receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver#file-log-receiver)
v0.139.0 version is [available as a Go
module](https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver@v0.139.0)
because we pushed the
[`receiver/filelogreceiver/v0.139.0`](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
git tag to this repository. We recommend you make a release at least every time there is a breaking
change on any of the Go modules you depend on.

### Distributing your component

Finally, to distribute your component and make sure others can easily discover it, add it to the
[OpenTelemetry registry](https://opentelemetry.io/ecosystem/registry/adding/).

If you think your component fits the requisites to be in this repository, you may choose to donate
it following the steps on the next section. Your activity, community, and popularity within your own
repository will help you make the case for the component to be accepted.

## Adding your component to the [opentelemetry-collector-contrib repository](https://github.com/open-telemetry/opentelemetry-collector-contrib)

After you have gotten some usage of your component outside of the [opentelemetry-collector-contrib
repository](https://github.com/open-telemetry/opentelemetry-collector-contrib), you can contribute
it to this repository. When you are ready to propose adding your component to this repository, [open
an
issue](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/new?assignees=&labels=Sponsor+Needed%2Cneeds+triage&projects=&template=new_component.yaml&title=Component+donation%3A+)
providing the following information:

* The GitHub handle of the **sponsor** for your component. A sponsor is an [approver or
  maintainer](https://github.com/open-telemetry/opentelemetry-collector-contrib?tab=readme-ov-file#contributing)
  who will be the official reviewer of the code and may also be a code owner. You will need to get
  consent from this individual to volunteer as a sponsor for your component. The sponsor must be
  from a different company than you. You can use the #otel-collector-dev CNCF Slack channel and the
  Collector SIG meetings to announce your intention to donate a component and see if any maintainers
  or approvers would be interested in sponsoring it. Please note that it is not guaranteed you will
  find a sponsor, as it depends on the availability of approvers and maintainers to assume
  responsibility for reviewing the component and maintaining it. If you are unable to find a
  sponsor, you can still continue to use your component as published in your own repository.
* The GitHub handles of the **codeowners** for your component. Codeowners are responsible for the component and
  will be pinged for any issues or reviews needed. You need at least three codeowners for your
  component to be accepted, one of which must be an approver or maintainer (which can be the
  sponsor). We recommend that the codeowners do not work for the same company.
* A **list of other components that cover similar use cases**. These could be components inside of
  this repository or outside of it. If there are competing implementations, invite their maintainers
  to voice their opinion and collaborate on a common implementation.
* Some **information about your component**, such as the reasoning behind it, use-cases, telemetry data
  types supported, and anything else you think is relevant for us to make a decision about accepting
  the component.
* The **configuration options** your component will accept. This will give us a better understanding of
  what it does, and how it may be implemented.

> [!IMPORTANT]  
> Unstable Collector interfaces may undergo breaking changes. Codeowners must be available to update
> or review their components when such changes happen, otherwise the component will be excluded from
> the default builds.

Maintenance of components is the responsibility of the code owners. [Unmaintained
components](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#unmaintained)
will be excluded from the default build. The component **will** be excluded if it causes build
problems, has failing tests, or otherwise causes problems to the rest of the repository and its
contributors.

### Donation

Once your donation is accepted, your component will undergo a review process that may imply changes to your component.
This is expected and it ensures that your component is aligned with the community's and this repo's best practices.

To donate your component to this repository, follow these steps:

- Add your component under the proper folder and use Go standard package naming recommendations.
- Use a boiler-plate Makefile that just references the one at top level, ie.: `include ../../Makefile.Common` - this
  allows you to build your component with required build configurations for the contrib repo while avoiding building the
  full repo during development.
- Each component has its own go.mod file. This allows custom builds of the collector to take a limited sets of
  dependencies - so run `go mod` commands as appropriate for your component.
- Make sure your code follows the pattern of existing components regarding config and factory source files and tests.
- Provide end-to-end tests (or mock backend/client as appropriate). Target is to get 80% or more of code coverage.
- Add a README.md on the root of your component describing its configuration and usage, likely referencing some of the
  yaml files used in the component tests. We also suggest that the yaml files used in tests have comments for all
  available configuration settings so users can copy and modify them as needed.
- Run `make crosslink` to update intra-repository dependencies. It will add a `replace` directive to `go.mod` file of every intra-repository dependant. This is necessary for your component to be included in the contrib executable.
- Add your component to `versions.yaml`.
- All components included in the distribution must be included in
  [`cmd/otelcontribcol/builder-config.yaml`](../cmd/otelcontribcol/builder-config.yaml)
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

//go:generate make mdatagen

// Package fooreceiver bars.
package fooreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fooreceiver"
```
- Type `make generate`. This will trigger the [metadata generator](https://github.com/open-telemetry/opentelemetry-collector/blob/main/cmd/mdatagen/README.md#using-the-metadata-generator) to generate the associated code/documentation.
- Type `make gencodeowners`. This will trigger the regeneration of the `.github/CODEOWNERS` file. 

When donating a component to the community, break it down into separate PRs as follows:

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

> [!IMPORTANT]  
> An 'in development' component that does not make progress towards alpha stability may be
> considered [unmaintained](https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#unmaintained) and eventually removed.

## Releasing New Components

After a component has been merged it must be added to the
[OpenTelemetry Collector Contrib's release manifest.yaml](https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/distributions/otelcol-contrib/manifest.yaml)
to be included in the distributed otelcol-contrib binaries and docker images.

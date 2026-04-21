# `pkg` folder

The Go modules in this folder can help in building new components (receivers, processors, exporters, extensions...) and are meant for **external** consumption.
Carefully review the public API to ensure that we only expose what is deemed necessary, so that we can easily change the code without affecting external users.

Breaking changes can happen on any module on this folder under the [opentelemetry-collector contribution guidelines](https://github.com/open-telemetry/opentelemetry-collector/blob/main/CONTRIBUTING.md#breaking-changes).

If you are introducing a new module that will only be used by components in the opentelemetry-collector-contrib repository, consider placing it on the `internal` folder instead.
This will reduce our public API footprint and maintenance burden.

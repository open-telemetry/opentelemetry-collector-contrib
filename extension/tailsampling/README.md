# Tail Sampling Extensions

Tail sampling extensions are a collection of custom sampling policy types that
can be enabled for use. They intent is to include more specific samplers that
may not make sense in all environments, or more experimental samplers that do
not make sense to include in the core component yet.

## Adding new extensions

Follow the instructions to create a new component from
[CONTRIBUTING.md](https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/CONTRIBUTING.md#adding-new-components).
You can use `examplesampler` as a reference for `factory.go` and the general
pattern of creating a sampler extension.

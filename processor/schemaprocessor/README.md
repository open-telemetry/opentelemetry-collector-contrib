# Schema Transformer Processor

🚧 _Currently under development, subject to change_ 🚧

Supported Pipelines: traces, metrics, logs

The _Schema Processor_ is used to convert existing telemetry data or signals to a version of the semantic convention defined as part of the configuration.
The processor works by using a set of target schema URLs that are used to match incoming signal.
On a match, the processor will fetch the schema translation file (if not cached) set by the incoming signal and apply transformations
required to export as the target semantic convention version.

Furthemore, it is also possible for organisations and vendors to publish their own semantic convention and be used by this processor, 
be sure to follow [schema overview](https://opentelemetry.io/docs/reference/specification/schemas/overview/) for all the details.

## Caching Schema Translation Files

In order to improve effeicency of the processor, the `precache` option allows the processor to start downloading and preparing
the translations needed for signals that match the schema url.

## Targets Schemas

Targets define a list of schema urls with a schema identifer that will be used to translate any schema url that matches the target url to that version.
In the event that the processor matches a signal to a target, the processor will translate the signal from published one to the defined identifier;
using the configuration bellow, a signal published with `SchemaURL:https://opentelemetry.io/schemas/1.8.0` will be translated by the collector to `SchemaURL:https//opentelemetry.io/schemas/1.6.1`.


# Example

```yaml
processors:
  schema_transformer:
    precache:
    - https://opentelemetry.io/schemas/1.9.0
    targets:
    - https://opentelemetry.io/schemas/1.6.1
```

For more complete examples, please refer to [config.yml](./testdata/config.yml).
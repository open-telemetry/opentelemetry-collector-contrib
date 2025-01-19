# OpenTelemetry Transformation Language Contexts

The OpenTelemetry Transformation Language uses Contexts to bridge the gap between a [Path](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/pkg/ottl#paths) and an underlying data model.

A Context must provide a struct that implements `ottl.TransformContext`.  A context must also define a `PathExpressionParser` and a `EnumParser`.

A Context's `PathExpressionParser` is what the OTTL will use to interpret a Path.  For the data model being represented, it should be able to handle any incoming path and return a `GetSetter` that will be able to accurately interact with the path's corresponding field.  It should return an error if the Path is not known.  The `GetSetter` functions will use the Context's implementation of `ottl.TransformContext` to interact with the correct item(s) during processing.

A Context's `EnumParser` is what the OTTL will use to interpret an Enum Symbol.  For the data model being represented, it should be able to handle any incoming Enum Symbol and return the appropriate Enum value.  It should return an error if the Enum Symbol is not known.  

Context implementations for Traces, Metrics, and Logs are provided by this module.  It is recommended to use these contexts when using the OTTL to interact with OpenTelemetry traces, metrics, and logs. 
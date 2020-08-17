# Dynamic Configuration Service
The dynamic configuration service implements a system in which telemetry
configurations can be updated at runtime via communication with a
configuration service. Currently, on metric schedules are supported.

**Note: this feature is experimental. Use at your own risk.**

## Resources
For an explanation of this system, please see [the experimental
specification](https://github.com/open-telemetry/opentelemetry-specification/blob/master/experimental/metrics/config-service.md).

To see it in action, check out [this example](https://github.com/vmingchen/opentelemetry-go-contrib/tree/master/sdk/dynamicconfig/example).

Its counterpart on the Go contrib SDK is available [here](https://github.com/open-telemetry/opentelemetry-go-contrib/pull/223).

## Running the integration test
An integration test suite is included with this component. To run it,
ensure you are in the directory `integration_test`, and do

```sh
go test -tags=integration -v
```

It will take ~12 minutes to run.

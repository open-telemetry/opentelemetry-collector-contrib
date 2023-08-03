# Kubernetes Test Helpers

The purpose of this package is to provide reusable kubernetes test helpers.

## Functionality

This package includes the ability to [create and delete k8s objects](./k8s_objects.go),
[run the collector](./k8s_collector.go), [check data received](./k8s_data_helpers.go) from the collector,
as well as [create telemetry generators](./k8s_telemetrygen.go), all within
a k8s environment.

## Example usage

Please refer to the k8sattributes processor's [e2e test](../../processor/k8sattributesprocessor/e2e_test.go)
for an example of how to use this package.
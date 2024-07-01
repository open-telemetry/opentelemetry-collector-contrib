# AWS Application Signals Processor

The AWS Application Signals processor is used to reduce the cardinality of telemetry metrics and traces before exporting them to CloudWatch Logs via [EMF](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/awsemfexporter) and [X-Ray](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/awsxrayexporter) respectively.
It reduces the cardinality of metrics and traces using three types of actions, `keep`, `drop` and `replace`, which are configured by users. Users can configure these rules through configurations.

Note: Traces support only `replace` actions and are implicitly pulled from the logs section of the configuration.

| Status                   |                           |
| ------------------------ |---------------------------|
| Stability                | [In Development](https://github.com/open-telemetry/opentelemetry-collector#development)          |
| Supported pipeline types | traces, metrics, logs     |
| Distributions            | []                        |


## Overview

This processor is currently under development and is presently a **NOP (No Operation) processor**. Further features and functionalities will be added in upcoming versions.

## (In Development) Overview

The AWS Application Signals processor provides a data-processing pipeline which has 4 major components:

1. `AttributesResolver`: Automatically resolves telemetry attributes with high cardinality values (e.g. Pod IPs from K8s/EKS) into static values. The AttributesResolver is an abstract interface which can be extended into different sub-resolvers. For example, we implemented K8sResolver to resolve the internal IP addresses/aliases in an EKS cluster with the corresponding service deployment name. We also plan to implement PublicIpResolver to resolve the public IP addresses into the static domain name.
2. `AttributesNormalizer`: Normalizes OTel attribute names based on the Application Signals Metrics & Traces schema definition. For example, renaming OTel metric attribute names (e.g. aws.remote.service to RemoteService).
3. `MetricLimiter`: Caps the total number of unique metrics a service can send. A limit is imposed on metrics generated for each service (by service.name) so that any additional metrics beyond the threshold limit will be dropped. The existing metrics prior to the threshold will be still be sent.
4. `CustomReplacer`: Reads the customer provided configuration rules to replace the attribute values based on the rules.
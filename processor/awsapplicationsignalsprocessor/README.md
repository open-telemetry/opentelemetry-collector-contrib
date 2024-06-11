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
# Apache Spark Receiver

| Status                   |                  |
| ------------------------ | ---------------- |
| Stability                | [in development] |
| Supported pipeline types | metrics          |
| Distributions            | [contrib]        |

This receiver fetches metrics from an Apache Spark application. Metrics are collected
based upon different configurations in the config file.

## Purpose

The purpose of this component is to allow monitoring of Apache Spark applications and jobs through the collection of performance metrics like memory utilization, CPU utilization, executor duration, etc. gathered through the exposed Spark REST API.

## Prerequisites

This receiver supports Apache Spark versions:

- 3.3.2+

## Configuration

### Connection Configuration

These configuration options are for connecting to an Apache Spark application.

- `collection_interval`: (default = `15s`): This receiver collects metrics on an interval. This value must be a string readable by Golang's [time.ParseDuration](https://pkg.go.dev/time#ParseDuration). Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.
- `endpoint`: (default = `http://localhost:4040`): Apache Spark endpoint to connect to in the form of `[http][://]{host}[:{port}]`

### Example Configuration

```yaml
receivers:
  apachespark:
    collection_interval: 60s
    endpoint: http://localhost:4040
```

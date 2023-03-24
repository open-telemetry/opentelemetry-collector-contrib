# Apache Pulsar Receiver

| Status                   |                  |
| ------------------------ | ---------------- |
| Stability                | [in development] |
| Supported pipeline types | metrics          |
| Distributions            | [contrib]        |

This receiver fetches stats from a Pulsar cluster using a [golang pulsar package](https://pkg.go.dev/github.com/streamnative/pulsarctl@v0.5.0/pkg/pulsar).

## Purpose

New Dev Project by Caleb Hurshman (03/2023)
Not to be confused with Pulsar Receiver

The purpose of this receiver is to allow users to monitor metrics for a topic in an Apache Pulsar cluster.

## Default Metrics

The following metrics are emitted by default. Each of them can be disabled by applying the following configuration:

```yaml
metrics:
  <metric_name>:
    enabled: false
```

### topic.msgincount

Number of messages published for a topic in the last interval

| Unit     | Metric Type       | Value Type |
| -------- | ----------------- | ---------- |
| messages | Non-monotonic sum | Int        |

### topic.backlogsize

Total unconsumed message data for a topic

| Unit  | Metric Type       | Value Type |
| ----- | ----------------- | ---------- |
| bytes | Non-monotonic sum | Int        |

### topic.sub.unackedmsgs

Number of unacknowledged messages for a subscription

| Unit     | Metric Type       | Value Type |
| -------- | ----------------- | ---------- |
| messages | Non-monotonic sum | Int        |

### topic.avgmsgsize

Average size of messages in bytes in the last interval

| Unit  | Metric Type | Value Type |
| ----- | ----------- | ---------- |
| bytes | Gauge       | Int        |

### Example Configuation

```yaml

```

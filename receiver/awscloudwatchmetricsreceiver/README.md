# CloudWatch Metrics Receiver

| Status                   |               |
| ------------------------ | ------------- |
| Stability                | [development] |
| Supported pipeline types | metrics       |
| Distributions            | [contrib]     |

Receives Cloudwatch metrics from [AWS Cloudwatch](https://aws.amazon.com/cloudwatch/) via the [AWS SDK for Cloudwatch Logs](https://docs.aws.amazon.com/sdk-for-go/api/service/cloudwatchlogs/)

## Getting Started

This receiver uses the [AWS SDK](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html) as mode of authentication, which includes [Profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-profiles.html) and [IMDS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html) authentication for EC2 instances.

## Configuration

### Top Level Parameters

| Parameter       | Notes      | type   | Description                                                                                                                                                                                                                                                                       |
| --------------- | ---------- | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `region`        | *required* | string | The AWS recognized region string                                                                                                                                                                                                                                                  |
| `profile`       | *optional* | string | The AWS profile used to authenticate, if none is specified the default is chosen from the list of profiles                                                                                                                                                                        |
| `imds_endpoint` | *optional* | string | A way of specifying a custom URL to be used by the EC2 IMDS client to validate the session. If unset, and the environment variable `AWS_EC2_METADATA_SERVICE_ENDPOINT` has a value the client will use the value of the environment variable as the endpoint for operation calls. |
| `metrics`          | *optional* | `Metrics` | Configuration for metrics ingestion of this receiver                                                                                                                                                                                                                              |

### Metrics Parameters

| Parameter                | Notes        | type                   | Description                                                                                |
| ------------------------ | ------------ | ---------------------- | ------------------------------------------------------------------------------------------ |
| `poll_interval`          | `default=1m` | duration               | The duration waiting in between requests.                                                  |
| `max_events_per_request` | `default=50` | int                    | The maximum number of events to process per request to Cloudwatch                          |
| `groups`                 | *optional*   | `See Group Parameters` | Configuration for Log Groups, by default all Log Groups and Log Streams will be collected. |

### Group Parameters

`named` is the way to control and filter which metrics collected . They are mutually exclusive and are incompatible to be configured at the same time.

- `named`
  - This is a map of log group name to stream filtering options
    - `namespace`: (optional)
      - `metric_names`: A list of full log stream names to filter the discovered log groups to collect from.
      - `metric_name_prefixes`: A list of prefixes to filter the discovered log groups to collect from.


#### Named Example

```yaml
awscloudwatchmetrics:
  region: us-west-1
  poll_interval: 5m
  metrics:
    named:
      - namespace: /aws/eks/dev-0/cluster: 
        metric_names: [kube-apiserver-ea9c831555adca1815ae04b87661klasdj]
```

## Sample Configs


[alpha]:https://github.com/open-telemetry/opentelemetry-collector#alpha
[contrib]:https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib
[Issue]:https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/15667

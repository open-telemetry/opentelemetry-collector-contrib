# Authenticator - Sigv4

This extension provides Sigv4 authentication for making requests to AWS services. For more information on the Sigv4 process, please look [here](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html).

## Configuration

The configuration fields are as follows:

* `assume_role`: Specifies the configuration needed to assume a role
  * `arn`: **Optional**. The Amazon Resource Name (ARN) of a role to assume
  * `session_name`: **Optional**. The name of a role session
* `region`: **Optional**. The AWS region for AWS Sigv4
    * Note that an attempt will be made to obtain a valid region from the endpoint of the service you are exporting to
    * [List of AWS regions](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.RegionsAndAvailabilityZones.html)
* `service`: **Optional**. The AWS service for AWS Sigv4
    * Note that an attempt will be made to obtain a valid service from the endpoint of the service you are exporting to


```yaml
extensions:
  sigv4auth:
    assume_role:
      arn: "arn:aws:iam::123456789012:role/aws-service-role/access"

receivers:
  hostmetrics:
    scrapers:
      memory:

exporters:
  prometheusremotewrite:
    endpoint: "https://aps-workspaces.us-west-2.amazonaws.com/workspaces/ws-XXX/api/v1/remote_write"
    auth:
      authenticator: sigv4auth

service:
  extensions: [sigv4auth]
  pipelines:
    metrics:
      receivers: [hostmetrics]
      processors: []
      exporters: [prometheusremotewrite]
```

## Notes

* The collector must have valid AWS credentials as used by the [AWS SDK for Go](https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/#specifying-credentials)

# AWS X-Ray Proxy

##
The AWS X-Ray proxy accepts requests without any authentication of AWS signatures applied and forwards them to the
AWS X-Ray API, applying authentication and signing. This allows applications to avoid needing AWS credentials to enable
X-Ray, instead configuring the AWS X-Ray exporter and/or proxy in the OpenTelemetry collector and only providing the
collector with credentials.

Currently, only the X-Ray Remote Sampler uses this proxy when issuing sampling requests.

## Configuration

Example:

```yaml
extensions:
  awsxrayproxy:
    endpoint: 0.0.0.0:2000
    proxy_address: ""
    tls:
      insecure: false
      server_name_override: ""
    region: ""
    role_arn: ""
    aws_endpoint: ""
    local_mode: false
```

The default configurations below are based on the [default configurations](https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L99) of the existing X-Ray Daemon.

### endpoint (Optional)
The TCP address and port on which this proxy listens for requests.

Default: `0.0.0.0:2000`

### proxy_address (Optional)
Defines the proxy address that this extension forwards HTTP requests to the AWS X-Ray backend through. If left unconfigured, requests will be sent directly.
This will generally be set to a NAT gateway when the collector is running on a network without public internet.

### insecure (Optional)
Enables or disables TLS certificate verification when this proxy forwards HTTP requests to the AWS X-Ray backend. This sets the `InsecureSkipVerify` in the [TLSConfig](https://godoc.org/crypto/tls#Config). When setting to true, TLS is susceptible to man-in-the-middle attacks so it should be used only for testing.

Default: `false`

### server_name_override (Optional)
This sets the ``ServerName` in the [TLSConfig](https://godoc.org/crypto/tls#Config).

### region (Optional)
The AWS region this proxy forwards requests to. When missing, we will try to retrieve this value through environment variables or optionally ECS/EC2 metadata endpoint (depends on `local_mode` below).

### role_arn (Optional)
The IAM role used by this proxy when communicating with the AWS X-Ray service. If non-empty, the receiver will attempt to call STS to retrieve temporary credentials, otherwise the standard AWS credential [lookup](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials) will be performed.

### aws_endpoint (Optional)
The X-Ray service endpoint which this proxy forwards requests to.

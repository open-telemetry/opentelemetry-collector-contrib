# AWS X-Ray Receiver

**Status: alpha**

## Overview
The AWS X-Ray receiver accepts segments (i.e. spans) in the [X-Ray Segment format](https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html).
This enables the collector to receive spans emitted by the existing X-Ray SDK. [Centralized sampling](https://github.com/aws/aws-xray-daemon/blob/master/CHANGELOG.md#300-2018-08-28) is also supported via a local TCP port.

## Configuration

Example:

```yaml
receivers:
  aws_xray:
    version: 1.0.0
    endpoint: localhost:2000
    proxy_server:
      tcp_endpoint: localhost:2000
      proxy_address: ""
      no_verify_ssl: false
      region: ""
      role_arn: ""
      aws_endpoint: ""
      local_mode: false
```

The default configurations below are based on the [default configurations](https://github.com/aws/aws-xray-daemon/blob/master/pkg/cfg/cfg.go#L99) of the existing X-Ray Daemon.

### version
The version of the configuration schema for this receiver.

Default: `1.0.0`

### endpoint
The UDP address and port on which this receiver listens for X-Ray segment documents emitted by the X-Ray SDK.

Default: `localhost:2000`

### tcp_endpoint
The address and port on which this receiver listens for calls from the X-Ray SDK and relays them to the AWS X-Ray backend to get sampling rules and report sampling statistics.

Default: `localhost:2000`

### proxy_address
Defines the proxy address that the local TCP server forwards HTTP requests to AWS X-Ray backend through.

Default: `""`

### no_verify_ssl
Enables or disables TLS certificate verification when the local TCP server forwards HTTP requests to the AWS X-Ray backend.

Default: `false`

### region
The AWS region the local TCP server forwards requests to.

Default: `""`

### role_arn
The IAM role used by the local TCP server when communicating with the AWS X-Ray service.

Default: `""`

### aws_endpoint
The X-Ray service endpoint which the local TCP server forwards requests to.

Default: `""`

### local_mode
Determines whether the ECS instance metadata endpoint will be called or not. Set to `true` to skip EC2 instance metadata check.

Default: `false`
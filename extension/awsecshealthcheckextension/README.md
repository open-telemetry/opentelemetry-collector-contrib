# AWS ECS Health Check Extension

This new health check extension is used for ECS, it will store the exporter failures in a queue and 
when there is a http request from ECS, will respond a health status based on the number of failures stored. 
The failures in the queue will be rotated if expired than the time interval provided by the customer config.

# Configuration

The following settings are required:

- `endpoint` (default = `127.0.0.1:13134`): The host which could handle http request and provide http response.
- `interval` (default = `5m`): Time interval to check the number of failures, old failures which expired will be rotated.
- `exporter_error_limit` (default = `5`): The error number threshold to mark containers as healthy.

# Example

```yaml
aws_ecs_health_check:
    endpoint: 127.0.0.1:13134
    interval: "5m"
    exporter_error_limit: 5
```
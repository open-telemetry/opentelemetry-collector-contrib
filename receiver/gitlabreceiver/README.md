# GitLab Receiver

## Traces - Getting Started

Workflow tracing support is actively being added to the GitLab receiver.
This is accomplished through the processing of GitLab webhook
events for pipelines. The [`pipeline`](https://docs.gitlab.com/ee/user/project/integrations/webhook_events.html#pipeline-events) event payloads are then constructed into `trace`
telemetry.

Each GitLab pipeline, along with it's jobs, are converted
into trace spans, allowing the observation of workflow execution times,
success, and failure rates.

### Configuration

**IMPORTANT: At this time the tracing portion of this receiver only serves a health check endpoint.**

The WebHook configuration exposes the following settings:

* `endpoint`: (default = `localhost:8080`) - The address and port to bind the WebHook to.
* `path`: (default = `/events`) - The path for Action events to be sent to.
* `health_path`: (default = `/health`) - The path for health checks.
* `secret`: (optional) - The secret used to [validate the payload](https://docs.gitlab.com/ee/user/project/integrations/webhooks.html#custom-headers).
* `required_header`: (optional) - The required header key and value for incoming requests.

The WebHook configuration block also accepts all the [confighttp](https://pkg.go.dev/go.opentelemetry.io/collector/config/confighttp#ServerConfig)
settings.

An example configuration is as follows:

```yaml
receivers:
    gitlab:
        webhook:
            endpoint: localhost:19418
            path: /events
            health_path: /health
            secret: ${env:SECRET_STRING_VAR}
            required_header:
                key: "X-GitLab-Event"
                value: "pipeline"
```

For tracing, all configuration is set under the `webhook` key. The full set
of exposed configuration values can be found in [`config.go`](config.go).

# Prometheus Federation Endpoint Example

This example showcases how the agent can connect to an existing Prometheus deployment.

The example runs as a Docker Compose deployment. The collector can be configured to send various metrics to OpenTelemetry receivers.

To deploy the example, check out this git repository, open a terminal and in this directory type:
```bash
$> docker-compose up --build
```

Once active, metrics will be scraped and sent to the standard output of the OpenTelemetry Collector.

You can review them with the following command:
```bash
docker logs otelcollector
```

Additionally, you can consult the [Prometheus UI](http://localhost:9090) to see the metric data collected from the sample go program.

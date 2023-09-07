# OpenTelemetry Collector Splunk demo

This demonstration shows how the OpenTelemetry Collector can send data to a Splunk instance.

Splunk runs as part of this docker-compose demo to show metrics data from your machine.

To build and run the demo, switch to this directory and run

`docker-compose up`

Hit `http://localhost:18000` and log in with `admin`/`opentelemetry`, then go to the [analytics workspace](http://localhost:18000/en-US/app/search/analytics_workspace).

You will see a list of analytics exposed by the host metrics collector, which captures simple metrics exposed inside the OpenTelemetry Collector Docker container.
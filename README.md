# SumoLogic / OpenTelemetry Collector Contrib
This is a repository for OpenTelemetry Collector Contrib with additional Sumo Logic extensions. It is based
on the [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) and
[SumoLogic flavor](https://github.com/SumoLogic/opentelemetry-collector) of 
[core distribution of the Collector](https://github.com/open-telemetry/opentelemetry-collector).


## Docker Images
Docker images for all releases are published at https://hub.docker.com/r/sumologic/opentelemetry-collector

### Building docker locally

```
docker build -f cmd/otelcontribcol/Dockerfile -t otelcontribcol .
```

## Differences from the core release

SumoLogic version of OpenTelemetry Collector introduces a number of additions over the plain version:

* Extensions to [k8sprocessor](https://github.com/SumoLogic/opentelemetry-collector-contrib/tree/master/processor/k8sprocessor) 
  which include more tags being collected and field extraction enhancements
* A [sourceprocessor](https://github.com/SumoLogic/opentelemetry-collector-contrib/tree/master/processor/sourceprocessor) that 
  adds additional tags (mostly relevant to K8s environment) and provides some data filtering rules
* [Tail sampling processor](https://github.com/SumoLogic/opentelemetry-collector/tree/master/processor/samplingprocessor/tailsamplingprocessor) 
  extensions, which include *cascading* policy with two-pass rules for determining span budget for each of the defined rules
* Additional release schedule, which allows to quickly introduce bugfixes and extensions

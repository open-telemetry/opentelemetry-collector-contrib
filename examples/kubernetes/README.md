# OpenTelemetry Collector Demo

This demo is a sample app to build the collector and exercise its kubernetes logs scraping functionality.

## Build and Run

### Kubernetes

Switch to this directory and run following command: `kubectl apply -n <namespace> -f otel-collector.yaml`

#### Include/Exclude Specific Logs

Kubernetes logs are being stored in `/var/log/pods`.
Path to container logs is constructed using following pattern:

`/var/log/pods/<namespace>_<pod_name>_<pod_uid>/<container>/<restart_count>.log`

You can use it to manage from which containers do you want to include and exclude logs.

For example, to include all logs from `default` namespace, you can use following configuration:

```yaml
include:
    - /var/log/pods/default_*/*/*.log
exclude: []
```

### Docker Compose

Two steps are required to build and run the demo:

1. Build latest docker image in main repository directory `make docker-otelcontribcol`
1. Switch to this directory and run `docker-compose up`

#### Description

`varlogpods` contains example log files placed in kubernetes-like directory structure.
Each of the directory has different formatted logs in one of three formats (either `CRI-O`, `CRI-Containerd` or `Docker`).
This directory is mounted to standard location (`/var/log/pods`).

`otel-collector-config` is a configuration to autodetect and parse logs for all of three mentioned formats

## Performance Tests

There are multiple tests for various configurations in [`testbed`](../../testbed/tests/log_test.go).
Following table shows result of example run:

Test                                    |Result|Duration|CPU Avg%|CPU Max%|RAM Avg MiB|RAM Max MiB|Sent Items|Received Items|
----------------------------------------|------|-------:|-------:|-------:|----------:|----------:|---------:|-------------:|
Log10kDPS/OTLP                          |PASS  |     15s|    10.4|    11.0|         68|         94|    150100|        150100|
Log10kDPS/filelog                       |PASS  |     15s|     7.8|     8.7|         65|         93|    150100|        150100|
Log10kDPS/kubernetes_containers         |PASS  |     15s|    18.3|    19.7|         67|         96|    150100|        150100|
Log10kDPS/kubernetes_containers_parser  |PASS  |     15s|    18.2|    19.0|         66|         95|    150100|        150100|
Log10kDPS/k8s_CRI-Containerd            |PASS  |     15s|    15.4|    16.3|         66|         95|    150100|        150100|
Log10kDPS/k8s_CRI-Containerd_no_attr_ops|PASS  |     15s|    15.1|    16.3|         66|         95|    150100|        150100|
Log10kDPS/CRI-Containerd                |PASS  |     15s|    11.1|    13.0|         65|         93|    150100|        150100|

## ToDo

To cover kubernetes system logs, logs from journald should be supported as well.

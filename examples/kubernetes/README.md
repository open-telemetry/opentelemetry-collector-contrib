# OpenTelemetry Collector Demo

This demo is a sample app to build the collector and exercise its kubernetes logs scrapping functionality.

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
Log10kDPS/OTLP                          |PASS  |     15s|    15.2|    15.7|         69|         73|    149900|        149900|
Log10kDPS/filelog                       |PASS  |     15s|    16.5|    18.0|         61|         74|    150000|        150000|
Log10kDPS/kubernetes_containers         |PASS  |     15s|    42.3|    44.0|         66|         80|    150000|        150000|
Log10kDPS/k8s_CRI-Containerd            |PASS  |     15s|    36.7|    38.0|         64|         78|    150000|        150000|
Log10kDPS/k8s_CRI-Containerd_no_attr_ops|PASS  |     15s|    28.9|    29.7|         64|         77|    150000|        150000|
Log10kDPS/CRI-Containerd                |PASS  |     15s|    19.0|    21.0|         63|         77|    150000|        150000|

## ToDo

To cover kubernetes system logs, logs from journald should be supported as well.

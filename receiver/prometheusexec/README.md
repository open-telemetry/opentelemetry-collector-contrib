# This receiver is currently under development, do not use.
// TODO: Remove above message when done

# Prometheus_exec Receiver

This receiver makes it easy for a user to collect metrics with the Collector from third-party services such as MySQL, Apache, Redis, etc. It's meant for people who want a plug-and-play solution to getting metrics from those third-party services that don't natively export metrics or speak any instrumentation protocols. Through the configuration file, you can indicate which binaries to run (usually [Prometheus exporters](https://prometheus.io/docs/instrumenting/exporters/), which are custom binaries that will expose the services' metrics to the Prometheus protocol). It supports starting binaries with all flags and environment variables, retrying them with exponentional backoff, string templating, and random port assigning. If you do not need to spawn the binaries locally, please condider using the [core Prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/prometheusreceiver) or the [Simple Prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/simpleprometheusreceiver).

## Overview
Here's an example Collector configuration file that starts two `prometheus_exec` receivers and two exporters.

```yaml
receivers:
    prometheus_exec/mysqld:
        exec: ./mysqld_exporter --port {{port}}
        scrape_interval: 10s
        port: 9104
        env:
          - name: DATA_SOURCE_NAME
            value: user:password@(url:port)/

    prometheus_exec/postgresql:
        exec: ./postgres_exporter
        env:
          - name: DATA_SOURCE_NAME
            value: postgresql://user:password@url:port/

exporters:
    stackdriver:
        project: "project_name"
    file:
        path: ./file_name.json

service:
    pipelines:
        metrics:
            receivers: [prometheus_exec/mysqld, prometheus_exec/postgresql]
            exporters: [stackdriver, file]
```

## Config
For every `prometheus_exec` defined in the configuration file, a command will be run (the user *should* want to start a binary) and an equivalent Prometheus receiver will be spawned to scrape its metrics.

- ### prometheus_exec (required)
`prometheus_exec` receivers should hierarchically be placed under the `receivers` key. As many of these receivers can be defined, and should be named as follows: `prometheus_exec/custom_name`. The `custom_name` should be unique per receiver you define, and is important for logging and error tracing. If no `custom_name` is given (i.e. `prometheus_exec`), one will try to be generated using the first word in the command to be executed. Example:

```yaml
receivers:
    prometheus_exec/mysql: # custom_name here is mysql

    prometheus_exec:
        exec: ./postgresql_exporter --port 1234 --config conf.xml # custome_name here is ./postgresql_exporter

```

- ### exec (required)
Under each `prometheus_exec/custom_name` there needs to be an `exec` key. The value of this key is a string of the command to be run, with any flags needed (this will probably be the binary to run, in the correct relative directory). Environment variables can be set in a later configuration setting. Example:

```yaml
receivers:
    prometheus_exec/apache:
        exec: ./apache_exporter --retry=false

    prometheus_exec/postgresql:
        exec: ./postgresql_exporter --port 1234 --config conf.xml

```

- ### port
`port` is an optional entry. Its value is a number indicating the port the receiver should be scraping the binary's metrics from. Two important notes about `port`:
1. If it is omitted, we will try to randomly generate a port for you, and retry until we find one that is free. Beware when using this, since you also need to indicate your binary to listen on that port with the use of a flag and templating inside the command, which is covered in 2.

2. **All** instances of `{{port}}` in any string of any key for the enclosing `prometheus_exec` will be replaced with either the port value indicated or the randomly generated one if no port value is set with the `port` key.

Example:

```yaml
receivers:
    prometheus_exec/apache:
        exec: ./apache_exporter --retry=false
        port: 1234 
    # this receiver will listen on port 1234

    prometheus_exec/postgresql:
        exec: ./postgresql_exporter --port {{port}} --config conf.xml
        port: 9876
    # this receiver will listen on port 9876 and {{port}} inside the command will become 9876

    prometheus_exec/mysql:
        exec: ./mysqld_exporter --port={{port}}
    # this receiver will listen on a random port and that port will be substituting the {{port}} inside the command
```

- ### scrape_interval
`scrape_interval` is an optional entry. Its value is a duration, in seconds (`s`), indicating how long the delay between scrapes done by the receiver is. The default is `10s` (10 seconds). Example:

```yaml
receivers:
    prometheus_exec/apache:
        exec: ./apache_exporter --retry=false
        port: 1234 
        scrape_interval: 60s
    # this receiver will scrape every 60 seconds

    prometheus_exec/postgresql:
        exec: ./postgresql_exporter --port {{port}} --config conf.xml
        port: 9876
    # this receiver will scrape every 10 seconds
```

- ### env
`env` is an optional entry to indicate which environment variables the command needs to run properly. Under it there should be a list of key (`name`) - value (`value`) pairs, separate by dashes (`-`). They are case-sensitive. These environment variables are added to the pre-existing environment variables the Collector is running with (the entire environment is replicated, including the directory). Example:

```yaml
receivers:
    prometheus_exec/mysql:
        exec: ./mysql_exporter --retry=false
        port: 1234 
        scrape_interval: 60s
        env:
          - name: DATA_SOURCE_NAME
            value: user:password@url:port/dbname
          - name: RETRY_TIMEOUT
            value: 10
    # this binary will start with the two above defined environment variables, 
```


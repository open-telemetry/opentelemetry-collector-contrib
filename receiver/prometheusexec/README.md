# This receiver is currently under development, do not use.
// TODO: Remove above message when done

# prometheus_exec Receiver

### Why?
This receiver makes it easy for a user to collect metrics from third-party services **via Prometheus exporters**. It's meant for people who want a plug-and-play solution to getting metrics from those third-party services that sometimes simply don't natively export metrics or speak any instrumentation protocols (MySQL, Apache, Nginx, etc.) while taking advantage of the large [Prometheus exporters]((https://prometheus.io/docs/instrumenting/exporters/)) ecosystem. 

### How?
Through the configuration file, you can indicate which binaries to run (usually [Prometheus exporters](https://prometheus.io/docs/instrumenting/exporters/), which are custom binaries that expose the third-party services' metrics using the Prometheus protocol) and `prometheus_exec` will take care of starting the specified binaries with their equivalent Prometheus receiver. This receiver also supports starting binaries with flags and environment variables, retrying them with exponentional backoff if they crash, string templating, and random port assignments.

*Note*: If you do not need to spawn the binaries locally, please consider using the [core Prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/master/receiver/prometheusreceiver) or the [Simple Prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/master/receiver/simpleprometheusreceiver).

## Config
For each `prometheus_exec` defined in the configuration file, the specified command will be run. The command *should* start a binary that exposes Prometheus metrics and an equivalent Prometheus receiver will be instantiated to scrape its metrics, if configured correctly.

- ### prometheus_exec (required)
`prometheus_exec` receivers should hierarchically be placed under the `receivers` key. You can define as many of these as you want, and they should be named as follows: `prometheus_exec/custom_name`. The `custom_name` should be unique per receiver you define, and is important for logging and error tracing. If no `custom_name` is given (i.e. simply having `prometheus_exec`), one will be generated for you by using the first word in the command to be executed (doing this limits the amount of receivers to 1 though since you can't define multiple receivers with duplicate names, so it's not recommended). Example:

```yaml
receivers:
    prometheus_exec/mysql: 
        exec: ./mysqld_exporter
        # custom_name here is "mysql"

    prometheus_exec:
        exec: ./postgres_exporter
        # custom_name here is "./postgres_exporter"
```

- ### exec (required)
Under each `prometheus_exec/custom_name` there needs to be an `exec` key. The value of this key is a string of the command to be run, with any flags needed (this will probably be the binary to run, in the correct relative directory). Environment variables can be set in a later configuration setting. Example:

```yaml
receivers:
    prometheus_exec/apache:
        exec: ./apache_exporter --log.level="info"

    prometheus_exec/postgresql:
        exec: ./postgres_exporter --web.telemetry-path="/metrics"
```

- ### port
`port` is an optional entry. Its value is a number indicating the port the receiver should be scraping the binary's metrics from. Two important notes about `port`:
1. If it is omitted, we will try to randomly generate a port for you, and retry until we find one that is free. Beware when using this, since you also need to indicate your binary to listen on that same port with the use of a flag and string templating inside the command, which is covered in 2.

2. **All** instances of `{{port}}` in any string of any key for the enclosing `prometheus_exec` will be replaced with either the port value indicated or the randomly generated one if no port value is set with the `port` key.

Example:

```yaml
receivers:
    prometheus_exec/apache:
        exec: ./apache_exporter
        port: 9117 
    # this receiver will listen on port 9117

    prometheus_exec/postgresql:
        exec: ./postgres_exporter --web.listen-address=":{{port}}"
        port: 9187
    # this receiver will listen on port 9187 and {{port}} inside the command will become 9187

    prometheus_exec/mysql:
        exec: ./mysqld_exporter --web.listen-address=":{{port}}"
    # this receiver will listen on a random port and that port will be substituting the {{port}} inside the command
```

- ### scrape_interval
`scrape_interval` is an optional entry. Its value is a duration, in seconds (`s`), indicating how long the delay between scrapes done by the receiver is. The default is `10s` (10 seconds). Example:

```yaml
receivers:
    prometheus_exec/apache:
        exec: ./apache_exporter
        port: 9117 
        scrape_interval: 60s
    # this receiver will scrape every 60 seconds

    prometheus_exec/postgresql:
        exec: ./postgres_exporter --web.listen-address=":{{port}}"
        port: 9187
    # this receiver will scrape every 10 seconds, by default
```

- ### env
`env` is an optional entry to indicate which environment variables the command needs to run properly. Under it, there should be a list of key (`name`) - value (`value`) pairs, separate by dashes (`-`). They are case-sensitive. When running a command, these environment variables are added to the pre-existing environment variables the Collector is currently running with (the entire environment is replicated, including the directory). Example:

```yaml
receivers:
    prometheus_exec/mysql:
        exec: ./mysqld_exporter 
        port: 9104 
        scrape_interval: 60s
        env:
          - name: DATA_SOURCE_NAME
            value: user:password@(hostname:port)/dbname
          - name: SECONDARY_PORT
            value: {{port}}
    # this binary will start with the two above defined environment variables, notice how string templating also works in env
```


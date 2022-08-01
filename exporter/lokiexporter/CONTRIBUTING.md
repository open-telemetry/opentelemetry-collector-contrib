Contributing to the Loki Exporter
===

In order to contribute to this exporter, it might be useful to have a local setup of Loki, or a free account
at Grafana Labs.

Local Loki
---

* Download and unpack the Loki package on a local directory, like `~/bin`
* Download a [config file](https://raw.githubusercontent.com/grafana/loki/master/cmd/loki/loki-local-config.yaml) and store as `config.yaml` on the local directory
* Start Loki with `$ loki-linux-amd64`, making sure the config file is on the current directory

Now, we need a tool to interact with Loki. Either use Grafana and configure it with a Loki data source, or use `logcli`:

* Download and unpack the `logcli` package on a local directory, like `~/bin`
* You are now ready to issue queries to Loki, such as: `logcli-linux-amd64 query '{exporter="OTLP"}'`

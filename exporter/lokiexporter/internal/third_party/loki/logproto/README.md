# Notice

This code was copied from https://github.com/grafana/loki/tree/master/pkg/logproto, due to incompatible dependency 
issues when this exporter was added to the top level binary (top level go.mod replace, even with a specific version,
is not a good option due to other components' dependency requirements).

https://github.com/grafana/loki-client-go was recently started, but is marked experimental and has not released a 
version yet. Once this project has released a supported version, we should evaluate switching to it.

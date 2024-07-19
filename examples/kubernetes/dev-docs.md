# Running OpenTelemetry Collector from source on k8s

Developing Collector's features that target to run on a Kubernetes environment require some special handling
for building and running the patches locally.


In order to build the Collector from source and deploy it on a local kind cluster
(a kind cluster is required to be installed already) use the following Makefile targets:

#### Build the Collector
`make kind-build`

#### Install the Collector as Daemonset
`make kind-install-daemonset`

This command will install the Collector using the [`daemonset-collector-dev.yaml`](./daemonset-collector-dev.yaml)
configuration sample.
This only stands as a sample configuration and users need to tune this according to their needs.

#### Uninstall the Daemonset
`make kind-uninstall-daemonset`

#### Install the Collector as Deployment
`make kind-install-deployment`

This command will install the Collector using the [`deployment-collector-dev.yaml`](./deployment-collector-dev.yaml)
configuration sample.
This only stands as a sample configuration and users need to tune this according to their needs.

#### Uninstall the Deployment
`make kind-uninstall-deployment`

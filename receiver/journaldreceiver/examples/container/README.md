# Running journaldreceiver in Docker and Kubernetes
The first step to run the journaldreceiver in Docker or Kubernetes is
to build a Docker image containing the Opentelemetry Collector and the
`journalctl` binary. The following `Dockerfile` can be used as a
starting point.

```dockerfile
FROM debian:13-slim

ARG OTEL_COL_VERSION=0.136.0

WORKDIR /opt

RUN apt update && \
    apt install -y systemd wget && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

RUN wget "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v${OTEL_COL_VERSION}/otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz" && \
    tar -xzf "otelcol-contrib_${OTEL_COL_VERSION}_linux_amd64.tar.gz"
COPY otelcol-config.yml /opt/otelcol-config.yml

ENTRYPOINT ["/opt/otelcol-contrib"]
CMD ["--config", "/opt/otelcol-config.yml"]
```

To build the image setting the Collector version at build time you can
use:
```sh
docker build --build-arg OTEL_COL_VERSION=0.136.0 -t otel-col-journald .
```

You can check the version of the Collector by running:
```sh
docker run --rm -it otel-col-journald:latest --version
```

To check the version of `journalctl` installed, run:
```
docker run --rm -it --entrypoint journalctl otel-col-journald:latest --version
```

## Docker
You can use the provided `otelcol-config.yml` to test the
journaldreceiver. If your journal logs are on `/var/log/journal`, run:

```sh
docker run --rm --mount type=bind,source=/var/log/journal,target=/opt/journal,ro otel-col-journald
```

You will see the journal logs from your machine being displayed by the
debug exporter.

## Kubernetes
The journaldreceiver can be used to ingest the node logs from a
Kubernets cluster, it is a very similar process as running it in
Docker: build the image, deploy a daemonset giving it access to the
journal folder on the nodes.

For this example we will use [Kind](https://kind.sigs.k8s.io/) to spin
up a Kubernetes cluster using Docker container nodes.

First build the docker image as shown above, then start the kind cluster:
```sh
kind create cluster
```

Load the image into the cluster:
```
kind load docker-image otel-col-journald:latest
```

Then deploy the Collector:
```
% kubectl apply -f manifest.yml
configmap/otel-config created
daemonset.apps/otel-collector created
```

Wait for the pod to be ready:
```
% kubectl get pods
NAME                   READY   STATUS    RESTARTS   AGE
otel-collector-k4fhv   1/1     Running   0          29s
```
Follow the pod logs to see the journaldreceiver working:
```
kubectl logs -f otel-collector-k4fhv # adjust the pod name
```

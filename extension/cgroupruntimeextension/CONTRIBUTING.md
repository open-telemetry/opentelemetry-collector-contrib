# Contributing to the Cgroup Go runtime extension

In order to contribute to this extension, it might be useful to have a working local setup.

## Testing

To run the integration tests locally for this extension, you can follow theses steps in a Linux environment.

Inside the extension folder, start a privileged docker container and share the code with the container

```bash
cd extension/cgroupruntimeextension
docker run -ti --privileged --cgroupns=host -v $(pwd):/workspace -w /workspace debian:bookworm-slim
```

Install Go and gcc to run the integration test

```bash
apt update && apt install -y wget sudo gcc && wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz && tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz && export PATH=$PATH:/usr/local/go/bin && go version && rm go1.23.4.linux-amd64.tar.gz
```

Run the integration test

```bash
CGO_ENABLED=1 go test -v -exec sudo -race -timeout 360s -parallel 4 -tags=integration,""
```

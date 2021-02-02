## Setup

If you are new to Go, it is recommended to work through the [How to Write Go Code](https://golang.org/doc/code.html) tutorial, which will ensure your Go environment is configured.

Clone this repo into your Go workspace:
```
cd $GOPATH/src
mkdir -p github.com/observiq && cd github.com/observiq
git clone git@github.com:observiq/stanza.git
cd $GOPATH/src/github.com/open-telemetry/opentelemetry-log-collection
```

## Building

To build the agent for another OS, run one of the following: 
```
make build // local OS
make build-windows-amd64
make build-linux-amd64
make build-darwin-amd64
```

## Running Tests

Tests can be run with `make test`.
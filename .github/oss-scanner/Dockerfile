FROM golang:1.26@sha256:4165ee6bf3b7811fd7b40373290405c83d0167dcb87ee5f29c427cdcef45ff8f AS build

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates git make \
    && rm -rf /var/lib/apt/lists/*

# scanner contract: the checkout lives inside the image, at /src
COPY . /src
WORKDIR /src

# This repository is several hundred Go modules with no go.work, so each one has to be downloaded on its own.
# `make otelcontribcol` only fetches what the collector binary needs, so without this the tests of every other module fail
# offline.
RUN set -e; for mod in $(find . -name go.mod -not -path './.git/*' | sort); do \
      (cd "$(dirname "$mod")" && go mod download); \
    done

RUN make otelcontribcol \
    && ln -s "/src/bin/otelcontribcol_$(go env GOOS)_$(go env GOARCH)" /usr/local/bin/otelcontribcol

# The build is complete and the scan has no network, so fail fast instead of waiting on proxy timeouts if something was
# missed above. GOTOOLCHAIN=local stops go from trying to fetch a newer toolchain.
ENV GOPROXY=off \
    GOTOOLCHAIN=local

# Compile the test binaries of every module so that they are cached for the offline scan, but do not run any test:
# running the whole suite takes hours here, and many components need a database, a container runtime or the network to
# pass. `-run '^$'` matches no test, so a module only fails here if it does not compile. Every module is built on its own,
# as the repository's own `make gotest` does. The full output goes to /opt/oss-scanner-go-test.log (outside /src, so the
# checkout stays clean); a failure only prints a warning here.
RUN for mod in $(find . -name go.mod -not -path './.git/*' | sort); do \
      dir="$(dirname "$mod")"; \
      [ -n "$(cd "$dir" && go list ./... 2>/dev/null)" ] || continue; \
      echo "=== $dir" >> /opt/oss-scanner-go-test.log; \
      (cd "$dir" && go test -count=1 -run '^$' ./... >> /opt/oss-scanner-go-test.log 2>&1) \
        || echo "WARNING: test binaries failed to build in $dir"; \
    done

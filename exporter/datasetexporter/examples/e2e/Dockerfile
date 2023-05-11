FROM golang:1.19

# install builder
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.77.0

# copy datasetexporter to run the latest version
COPY . ./datasetexporter/

# copy build configuration
COPY examples/e2e/otelcol-builder.yaml .

# build it
RUN $GOPATH/bin/builder --config=otelcol-builder.yaml

ENTRYPOINT ["otelcol-dataset/otelcol-dataset"]

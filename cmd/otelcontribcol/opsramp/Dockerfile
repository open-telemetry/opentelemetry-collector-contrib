# Builds otelcontribcol (including resourceuuidprocessor) for the eworker sidecar.
# Build from the repository root:
#   docker build -f cmd/otelcontribcol/opsramp/Dockerfile -t <registry>/otelcontribcol-opsramp:<tag> .
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN make genotelcontribcol
WORKDIR /src/cmd/otelcontribcol
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/otelcontribcol .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
 && mkdir -p /var/log/otelcol \
 && chown 10001:10001 /var/log/otelcol
COPY --from=build /out/otelcontribcol /otelcontribcol
COPY cmd/otelcontribcol/opsramp/run-otelcontribcol.sh /usr/local/bin/run-otelcontribcol.sh
RUN chmod 0755 /usr/local/bin/run-otelcontribcol.sh
USER 10001
ENTRYPOINT ["/usr/local/bin/run-otelcontribcol.sh"]

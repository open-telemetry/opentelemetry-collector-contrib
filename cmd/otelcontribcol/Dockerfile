FROM golang:1.14.1 as builder
ADD . /src
WORKDIR /src
RUN make install-tools
RUN make common
RUN make otelcontribcol

FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /src/bin/otelcontribcol_linux_amd64 /otelcontribcol
EXPOSE 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

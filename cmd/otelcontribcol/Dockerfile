FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
ARG TARGETOS=linux
ARG TARGETARCH=amd64
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY otelcontribcol_${TARGETOS}_${TARGETARCH} /otelcontribcol
EXPOSE 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

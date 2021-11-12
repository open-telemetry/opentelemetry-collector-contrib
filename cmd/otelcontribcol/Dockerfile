FROM alpine:latest as prep
RUN apk --update add ca-certificates

ARG USER_UID=10001

RUN mkdir -p /tmp

FROM scratch

USER ${USER_UID}

COPY --from=prep /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY otelcontribcol /
EXPOSE 4317 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

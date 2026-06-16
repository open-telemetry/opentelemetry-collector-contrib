FROM alpine:latest@sha256:a2d49ea686c2adfe3c992e47dc3b5e7fa6e6b5055609400dc2acaeb241c829f4 AS prep
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

COPY --from=prep /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY otelcontribcol /
EXPOSE 4317 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

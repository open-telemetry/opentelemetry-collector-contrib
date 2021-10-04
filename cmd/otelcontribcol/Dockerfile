FROM alpine:latest as prep
RUN apk --update add ca-certificates

ARG USER_UID=10001

# github.com/containers/podman/v2 requires these directories to be present
# and be writable. When this is not satisfied, the library raises a fatal
# error and crashes the collector.
# This is required by the podmanreceiver.
RUN mkdir -p /rundir
RUN mkdir -p /tmp
RUN chown -R ${USER_UID}:${USER_UID} /rundir

FROM scratch

USER ${USER_UID}

COPY --from=prep /rundir /rundir
COPY --from=prep /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY otelcontribcol /
EXPOSE 4317 4318 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

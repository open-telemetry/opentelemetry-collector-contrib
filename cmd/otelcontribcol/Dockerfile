FROM debian:bookworm-slim

RUN apt-get update && \
  apt-get install -qy ca-certificates systemd && \
  rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ARG USER_UID=10001
USER ${USER_UID}

COPY otelcontribcol /
EXPOSE 4317 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

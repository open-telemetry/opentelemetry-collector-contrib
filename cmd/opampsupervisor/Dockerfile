FROM otelcontribcol AS col

FROM alpine:latest@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62 AS prep
RUN apk --update add ca-certificates
RUN mkdir -p /etc/otel/supervisor-data/

FROM scratch

ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

COPY --from=prep /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=prep --chmod=777 --chown=${USER_UID}:${USER_GID} /etc/otel/supervisor-data /etc/otel/supervisor-data
COPY --chmod=755 opampsupervisor /
COPY ./examples/supervisor_docker.yaml /etc/otel/supervisor.yaml
COPY --from=col --chmod=755 /otelcontribcol /otelcontribcol

EXPOSE 4317 55680 55679
ENTRYPOINT ["/opampsupervisor"]
CMD ["--config", "/etc/otel/supervisor.yaml"]

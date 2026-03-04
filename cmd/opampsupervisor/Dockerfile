FROM otelcontribcol AS col

FROM alpine:latest@sha256:4b7ce07002c69e8f3d704a9c5d6fd3053be500b7f1c69fc0d80990c2ad8dd412 AS prep
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

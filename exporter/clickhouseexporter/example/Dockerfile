FROM alpine:latest@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1 AS certs
RUN apk --update add ca-certificates

FROM golang:latest@sha256:fdcd2e5a34587bd5426c90e1531fd5ba448c89bb738df0f33860dfc69439a1f5

ARG USER_UID=10001
USER ${USER_UID}

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY ./otelcontribcol /otelcontribcol
ENTRYPOINT ["/otelcontribcol"]
EXPOSE 4317 55680 55679

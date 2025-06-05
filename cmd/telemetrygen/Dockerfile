FROM alpine:latest@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715 AS certs
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

ARG TARGETOS
ARG TARGETARCH

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY ./telemetrygen_${TARGETOS}_${TARGETARCH} /telemetrygen

ENTRYPOINT ["/telemetrygen"]

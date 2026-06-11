FROM alpine:latest@sha256:4f4ba248d8a2c90a6e52ffdfc194181f7617f9ddaca348d4c550a6b354fc7c2a AS certs
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
ARG USER_GID=10001
USER ${USER_UID}:${USER_GID}

ARG TARGETOS
ARG TARGETARCH

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY ./golden_${TARGETOS}_${TARGETARCH} /golden

ENTRYPOINT ["/golden"]

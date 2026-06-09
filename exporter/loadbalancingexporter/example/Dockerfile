FROM golang:1.26@sha256:68cb6d68bed024785b69195b89af7ac7a444f27791435f98647edff595aa0479 AS build

WORKDIR /src
ADD . /src

RUN make otelcontribcol

FROM alpine:latest@sha256:4f4ba248d8a2c90a6e52ffdfc194181f7617f9ddaca348d4c550a6b354fc7c2a AS certs
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
USER ${USER_UID}

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /src/bin/otelcontribcol_linux_amd64 /otelcontribcol
ENTRYPOINT ["/otelcontribcol"]
EXPOSE 4317 55680 55679

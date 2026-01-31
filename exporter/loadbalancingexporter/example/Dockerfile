FROM golang:1.25@sha256:ce63a16e0f7063787ebb4eb28e72d477b00b4726f79874b3205a965ffd797ab2 AS build

WORKDIR /src
ADD . /src

RUN make otelcontribcol

FROM alpine:latest@sha256:865b95f46d98cf867a156fe4a135ad3fe50d2056aa3f25ed31662dff6da4eb62 AS certs
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
USER ${USER_UID}

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /src/bin/otelcontribcol_linux_amd64 /otelcontribcol
ENTRYPOINT ["/otelcontribcol"]
EXPOSE 4317 55680 55679

FROM golang:1.25@sha256:0caf875670e0ec9ebe7f4a9f4cf02add9d06ffccb055cc1066c83270c237dfb9 AS build

WORKDIR /src
ADD . /src

RUN make otelcontribcol

FROM alpine:latest@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1 AS certs
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
USER ${USER_UID}

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /src/bin/otelcontribcol_linux_amd64 /otelcontribcol
ENTRYPOINT ["/otelcontribcol"]
EXPOSE 4317 55680 55679

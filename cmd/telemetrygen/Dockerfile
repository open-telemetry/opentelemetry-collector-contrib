FROM golang:1.20 AS build

WORKDIR /src
ADD . /src

RUN CGO_ENABLED=0 go build -o /telemetrygen

FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch

ARG USER_UID=10001
USER ${USER_UID}

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /telemetrygen /telemetrygen

ENTRYPOINT ["/telemetrygen"]

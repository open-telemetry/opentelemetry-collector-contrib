# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
FROM golang:1.24 AS build
WORKDIR /app/
COPY . .
RUN go env -w GOPROXY=direct
RUN CGO_ENABLED=0 go build -o main main.go
FROM alpine:3.21
COPY --from=build /app/main  /app/main
CMD ["/app/main"]

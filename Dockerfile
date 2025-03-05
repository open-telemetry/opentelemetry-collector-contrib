FROM golang:1.22
COPY  ./bin/ /
RUN ls .
ENTRYPOINT ["/otelcontribcol_amd64"]


# FROM europe-docker.pkg.dev/kyma-project/prod/external/library/golang:1.24.0-alpine3.21 as build
# ARG OTEL_VERSION
# ARG OTEL_CONTRIB_VERSION=${OTEL_VERSION}

# RUN apk --update add ca-certificates git

# ADD receiver /receiver/
# ADD internal /internal/
# ADD exporter /exporter/
# ADD extension /extension/
# WORKDIR /app
# COPY otel-collector/builder-config.yaml builder-config.yaml

# ENV OTEL_VERSION ${OTEL_VERSION}
# ENV OTEL_CONTRIB_VERSION ${OTEL_CONTRIB_VERSION}
# RUN sed -i s/OTEL_VERSION/${OTEL_VERSION}/g builder-config.yaml
# RUN sed -i s/OTEL_CONTRIB_VERSION/${OTEL_CONTRIB_VERSION}/g builder-config.yaml

# RUN go install go.opentelemetry.io/collector/cmd/builder@v${OTEL_VERSION}

# RUN builder --config=builder-config.yaml

# FROM scratch

# COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
# COPY --from=build --chmod=755 /app/kyma-otelcol /

# USER 65532:65532

# ENTRYPOINT ["/kyma-otelcol"]
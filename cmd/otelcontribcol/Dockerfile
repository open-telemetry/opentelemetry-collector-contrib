FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY ./xray-config.yaml /
COPY otelcontribcol /
EXPOSE 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/xray-config.yaml"]

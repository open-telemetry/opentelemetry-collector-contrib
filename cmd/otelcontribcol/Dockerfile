# When updating, check if similar changes need to be made to the top-level Dockerfile
FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY otelcontribcol /
ENTRYPOINT ["/otelcontribcol"]
EXPOSE 55680 55679

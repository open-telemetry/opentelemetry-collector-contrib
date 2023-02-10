FROM ubuntu:latest as prep
RUN apt-get update
RUN apt-get install ca-certificates -y
RUN update-ca-certificates

RUN mkdir -p /tmp

COPY otelcontribcol /
EXPOSE 4317 55680 55679
ENTRYPOINT ["/otelcontribcol"]
CMD ["--config", "/etc/otel/config.yaml"]

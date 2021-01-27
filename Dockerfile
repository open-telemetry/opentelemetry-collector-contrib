FROM ubuntu:bionic

RUN mkdir -p /stanza_home
ENV STANZA_HOME=/stanza_home
RUN echo "pipeline:\n" >> /stanza_home/config.yaml
RUN apt-get update && apt-get install -y systemd ca-certificates

COPY ./artifacts/stanza_linux_amd64 /stanza_home/stanza
COPY ./artifacts/stanza-plugins.tar.gz /tmp/stanza-plugins.tar.gz
RUN tar -zxvf /tmp/stanza-plugins.tar.gz -C /stanza_home/
ENTRYPOINT /stanza_home/stanza \
  --config /stanza_home/config.yaml \
  --database /stanza_home/stanza.db \
  --plugin_dir /stanza_home/plugins

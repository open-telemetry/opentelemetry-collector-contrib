FROM nvidia/cuda:10.2-devel-ubuntu18.04

LABEL maintainer "OpenTelemetry Authors"

ENV DEBIAN_FRONTEND noninteractive
ENV GO_VER 1.14.4
ENV PATH /usr/local/go/bin:$HOME/go/bin:${PATH}

RUN apt-get update && apt-get install -y --no-install-recommends \
     ca-certificates \
     build-essential \
     curl \
     nvidia-driver-440 \
  && curl -L "https://dl.google.com/go/go${GO_VER}.linux-amd64.tar.gz" | tar xz -C /usr/local

WORKDIR /build

CMD ["/bin/bash"]
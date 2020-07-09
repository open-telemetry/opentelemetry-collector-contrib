FROM cimg/go:1.14

LABEL maintainer "OpenTelemetry Authors"

ENV NVIDIA_REPO_URL=https://developer.download.nvidia.com/compute/cuda/repos/ubuntu1804/x86_64
ENV NVIDIA_APT_KEY=7fa2af80.pub
ENV C_INCLUDE_PATH /usr/local/cuda-10.2/include
ENV LD_LIBRARY_PATH /usr/local/cuda-10.2/targets/x86_64-linux/lib/stubs:${LD_LIBRARY_PATH}
RUN curl -fsSL "${NVIDIA_REPO_URL}/${NVIDIA_APT_KEY}" | sudo apt-key add - \
  && echo "deb ${NVIDIA_REPO_URL} /" > sudo tee /etc/apt/sources.list.d/cuda.list \
  && sudo apt-get update && sudo apt-get install --no-install-recommends \
      cuda-compat-10-2 \
      cuda-libraries-dev-10-2 \
      cuda-nvml-dev-10-2

WORKDIR /build

CMD ["/bin/bash"]
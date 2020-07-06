# CUDA (GPU) Receiver

**Status: alpha**

## Prerequisites

CUDA Receiver is currently only supported on the following environment.

* OS: Ubuntu 18.04
* CPU architecture: x86_64
* CUDA Toolkit: version 10.2

Technically, it depends upon `nvml.h` and `libnvidia-ml.so`, and it should work on the environment where CUDA Toolkit version 10.2 is available.

* https://developer.nvidia.com/cuda-10.2-download-archive

## Configuration

Example:

```
receivers:
  cudametrics:
    scrape_interval: 1m
    metric_prefix: "cuda102"
```

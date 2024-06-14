# Admission Package

## Overview

The admission package provides a BoundedQueue object which is a semaphore implementation that limits the number of bytes admitted into a collector pipeline. Additionally the BoundedQueue limits the number of waiters that can block on a call to `bq.Acquire(sz int64)`.

This package is an experiment to improve the behavior of Collector pipelines having their `exporterhelper` configured to apply backpressure. This package is meant to be used in receivers, via an interceptor or custom logic. Therefore, the BoundedQueue helps limit memory within the entire collector pipeline by limiting two dimensions that cause memory issues:
1. bytes: large requests that enter the collector pipeline can require large allocations even if downstream components will eventually limit or ratelimit the request.
2. waiters: limiting on bytes alone is not enough because requests that enter the pipeline and block on `bq.Acquire()` can still consume memory within the receiver. If there are enough waiters this can be a significant contribution to memory usage.

## Usage 

Create a new BoundedQueue by calling `bq := admission.NewBoundedQueue(maxLimitBytes, maxLimitWaiters)`

Within the component call `bq.Acquire(ctx, requestSize)` which will either
1. succeed immediately if there is enough available memory
2. fail immediately if there are too many waiters
3. block until context cancelation or enough bytes becomes available

Once a request has finished processing and is sent downstream call `bq.Release(requestSize)` to allow waiters to be admitted for processing. Release should only fail if releasing more bytes than previously acquired.
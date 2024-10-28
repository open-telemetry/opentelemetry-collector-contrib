# Admission Package

## Overview

The admission package provides a BoundedQueue object.  This object
implements a semaphore for limiting the number of bytes admitted into
a collector pipeline.  Additionally, the BoundedQueue limits the
number of bytes allowed to block on a call to `Acquire(pending int64)`.

There are two error conditions generated within this code:

- `rejecting request, too much pending data`: When the limit on waiting bytes its reached, this will be returned to limit the total amount waiting.
- `rejecting request, request is too large`: When an individual request exceeds the configured limit, this will be returned without acquiring or waiting.

The BoundedQueue implements LIFO semantics.  See this
[article](https://medium.com/swlh/fifo-considered-harmful-793b76f98374)
explaining why it is preferred to FIFO semantics.

## Usage 

Create a new BoundedQueue by calling `bq := admission.NewBoundedQueue(maxLimitBytes, maxLimitWaiters)`

Within the component call `bq.Acquire(ctx, requestSize)` which will either
1. succeed immediately if there is enough available memory
2. fail immediately if there are too many waiters
3. block until context cancelation or enough bytes becomes available

Once a request has finished processing and is sent downstream call `bq.Release(requestSize)` to allow waiters to be admitted for processing. Release should only fail if releasing more bytes than previously acquired.

# Blob Upload Connector

## Status

Under development.

This code is a skeleton only. Follow implementation at [#33737](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33737) ("New component: Blob Uploader Connector").

## What It Does

1. Queues an upload for specified pieces of a signal to a configured blob storage destination.
2. Replaces the original data with a reference to where the blob was written.
3. Forwards the modified signal to subsequent pipeline (to be exported per normal).

## Motivation

1. Handling overly large pieces of a signal that may be too big for a typical ops backend
2. Handling sensitive information by sending to an alternative system with tighter ACLs

## Config

The configuration will look roughly as follows:

```
blobuploadconnector:
  common: ...
  traces:  ...
  logs: ...
```

As shown above, there will be some set of common configuration that applies by default to all signal types.

Each signal type will have its own config for how it will be processed.

### Common Config

The common configuration governs things like how to upload the data:

```
blobuploadconnector:
  common:
    upload:
      queue_size: 100
      timeout_nanos: 5000
  ...
```

The `queue_size` indicates the maximum number of parallel uploads; the `timeout_nanos` specifies the timeout on uploading.

### Traces Config

The traces configuration governs how to match trace spans and span events. It will look roughly as follows:

```
blobuploadconnector:
  traces:
    # matching attributes on spans
    attributes: ...
    # matching (groups of) span events and their attributes
    events: ...
  ...
```

### Logs Config

The logs configuration governs how to match logs. It will look roughly as follows:

```
blobuploadconnector:
  logs:
    groups:
    - name: ...
      match: ...
      body: ...
      attributes: ...
  ...
```

Each named group will match logs based on `match`. For matched logs, `attributes` will govern whether/how to upload attributes while `body` will govern whether/how to upload portions of the body.

## See Also

- [#1428](https://github.com/open-telemetry/semantic-conventions/issues/1428) - "Seeking input on proposal for 'reference'-type attribute values"
- [#33737](https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33737) - "New component: Blob Uploader Connector"

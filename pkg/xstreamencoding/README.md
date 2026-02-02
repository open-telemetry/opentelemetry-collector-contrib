# Stream Helpers

> [!NOTE]
> These helpers are experimental and may change in future releases.

This package provides reusable helpers for stream-based unmarshaling of OpenTelemetry signals. 
It is designed to support efficient processing of **newline-delimited** streams with configurable batching and flushing behavior.

## Overview

When processing large data streams, it's often necessary to batch records and flush them periodically based on size or count thresholds.
This package provides the building blocks to implement such stream processing logic consistently across different data formats.

## Components

### ScannerHelper

A helper that wraps `io.Reader` to scan newline-delimited records.
User may forward a `bufio.Reader` with predefined buffers to optimize stream reading.
It tracks batch metrics and signals when to flush based on configured thresholds using `encoding.UnmarshalOption` functional options.

**Note:** Not safe for concurrent use.

### BatchHelper

A standalone helper for tracking batch metrics (bytes and items) and determining flush conditions.
Useful when you need custom scanning logic but still want batch tracking.

**Note:** Not safe for concurrent use.

### Unmarshaler Adapters

- `LogsDecoderFunc` - Adapts a function to the `encoding.LogsDecoder` interface
- `MetricsDecoderFunc` - Adapts a function to the `encoding.MetricsDecoder` interface

## Usage

### Flush batch by Item Count

```go
// Flush after every 100 items
helper := xstreamencoding.NewScannerHelper(reader,
    encoding.WithFlushItems(100),
)

for {
    line, flush, err := helper.ScanString()
    if err == io.EOF {
        break
    }
    if err != nil {
        return err
    }

    batch = append(batch, line)

    if flush {
        sendBatch(batch)
        batch = batch[:0] // Reset batch
    }
}
```

### Flush by Byte Size

```go
// Flush after accumulating ~1MB of data
helper := xstreamencoding.NewScannerHelper(reader,
    encoding.WithFlushBytes(1024 * 1024),
)
```

### Combined Flush Conditions

```go
// Flush after 1000 items OR 1MB, whichever comes first
helper := xstreamencoding.NewScannerHelper(reader,
    encoding.WithFlushItems(1000),
    encoding.WithFlushBytes(1024 * 1024),
)
```

### Using BatchHelper Standalone

For custom scanning logic where you need full control over reading records, you can use `BatchHelper` directly:

```go
batchHelper := xstreamencoding.NewBatchHelper(
    encoding.WithFlushItems(100),
    encoding.WithFlushBytes(1024 * 1024),
)

scanner := bufio.NewScanner(reader)
for scanner.Scan() {
    record := scanner.Bytes()

    // Track each record
    batchHelper.IncrementItems(1)
    batchHelper.IncrementBytes(int64(len(record)))

    // Check if batch should be flushed
    if batchHelper.ShouldFlush() {
        flushBatch()
        batchHelper.Reset()
    }
}
```

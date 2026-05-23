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
It tracks batch metrics and signals when to flush based on configured thresholds using `encoding.DecoderOption` functional options.
It also tracks the current byte offset read from the stream via `Offset()` method.
Use `Options()` to access the configured decoder options.

**Note:** Not safe for concurrent use.

### BatchHelper

A standalone helper for tracking batch metrics (bytes and items) and determining flush conditions.
Useful when you need custom scanning logic but still want batch tracking.
Use `Options()` to access the configured decoder options.

**Note:** Not safe for concurrent use.

### Decoder Adapters

- `LogsDecoderAdapter` - A struct that implements `encoding.LogsDecoder` interface by wrapping decode and offset functions
- `MetricsDecoderAdapter` - A struct that implements `encoding.MetricsDecoder` interface by wrapping decode and offset functions

Use `NewLogsDecoderAdapter` and `NewMetricsDecoderAdapter` to create instances.

## Usage

### Flush batch by Item Count

```go
// Flush after every 100 items
helper, err := xstreamencoding.NewScannerHelper(reader,
    encoding.WithFlushItems(100),
)

if err != nil {
    return nil, err
}

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
helper, err := xstreamencoding.NewScannerHelper(reader,
    encoding.WithFlushBytes(1024 * 1024),
)
```

### Combined Flush Conditions

```go
// Flush after 1000 items OR 1MB, whichever comes first
helper, err := xstreamencoding.NewScannerHelper(reader,
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

### Using Decoder Adapters

To implement `encoding.LogsDecoder` or `encoding.MetricsDecoder`, you may use the decoder adapter structs:

```go
func (c *myCodec) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
    scanner, err := xstreamencoding.NewScannerHelper(reader, options...)
    if err != nil {
        return nil, err
    }
	
    logs := plog.NewLogs()

    decodeFunc := func() (plog.Logs, error) {
        for {
            line, flush, err := scanner.ScanBytes()

            if len(line) > 0 {
                // Parse line and add to logs
                lr := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
                lr.Body().SetStr(string(line))
            }

            if flush || err == io.EOF {
                result := logs
                logs = plog.NewLogs() // reset for next batch
                if err == io.EOF {
                    return result, io.EOF
                }
                return result, nil
            }

            if err != nil {
                return plog.Logs{}, err
            }
        }
    }

    offsetFunc := func() int64 {
        return scanner.Offset()
    }

    return xstreamencoding.NewLogsDecoderAdapter(decodeFunc, offsetFunc), nil
}
```


# Contributing to the Filelog Receiver

## Testing

### Benchmarks

Performance benchmarks are available for the logs signal:
- Single file processing (1 to 100,000 lines)
- Multiple concurrent file processing (1-20 files)
- Receiver startup and shutdown
- Log parsing with different emission modes

**Latest benchmark results**: View the [benchmark workflow runs](https://github.com/open-telemetry/opentelemetry-collector-contrib/actions/workflows/build-and-test.yml) for performance metrics.

To run benchmarks locally:
```bash
go test -bench=. -benchtime=1s
```

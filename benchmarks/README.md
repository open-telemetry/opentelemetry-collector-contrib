# gnet vs gRPC Benchmarks

Measures throughput (ops/sec, MB/sec) and allocations of the gnet transport
against the standard OTLP/gRPC stack under identical payload sizes and
concurrency levels.

## Run

```bash
# All benchmarks, 10 seconds each
go test -bench=. -benchtime=10s -benchmem ./benchmarks/

# Individual comparisons
go test -bench=BenchmarkGnetTraces    -benchtime=10s -benchmem ./benchmarks/
go test -bench=BenchmarkGRPCTraces    -benchtime=10s -benchmem ./benchmarks/
go test -bench=BenchmarkGnetParallel  -benchtime=10s -benchmem -cpu=8 ./benchmarks/
go test -bench=BenchmarkGRPCParallel  -benchtime=10s -benchmem -cpu=8 ./benchmarks/

# CPU profile for flame graph
go test -bench=BenchmarkGnetParallel -cpuprofile=cpu_gnet.pprof ./benchmarks/
go test -bench=BenchmarkGRPCParallel -cpuprofile=cpu_grpc.pprof ./benchmarks/
go tool pprof -http=:6060 cpu_gnet.pprof
```

## What to measure

| Metric | Tool |
|---|---|
| ops/sec, MB/sec | `go test -bench` output |
| p50/p99 latency | wrap calls with `time.Now()` in a custom test |
| CPU% per signal | `cpuprofile` + pprof flamegraph |
| Allocs/op | `-benchmem` output |

## Expected results (hypothesis)

gnet should win on **raw throughput** (fewer syscalls, no HTTP/2 framing, no
reflection, no header compression overhead) but gRPC has better built-in
**flow control**, **TLS**, and **load balancing**. The benchmark will show
where the crossover is.

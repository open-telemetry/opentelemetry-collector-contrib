# Series Tracking

Delta conversion must track the last known cumulative value on a per-series
basis. A hash-map provides that requirement.

The Go `map` implementation is not thread-safe. As of 2025/01/28
deltatocumulative uses a global `sync.Mutex`. This is inefficient when load
increases, as it effectively serializes client requests.

To relieve this bottleneck, one could switch from writing values to the map to
having the map track mutable values, so everything but initial series creation
turns into a read. This in turn requires the values themselves to be thread safe.

To decide on an alternative, some research was done using benchmarks.

## Map

Using atomic values:

```
$ benchstat -col /map -row "/value,/gomaxprocs" -filter /value:/atomic/ map.bench
goos: linux
goarch: amd64
pkg: github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/research/tracking
cpu: Intel(R) Xeon(R) CPU @ 2.80GHz
                │ std+prealloc │              std+global              │              std+RWMutex               │                sync.Map                │              xsync.MapOf              │
                │    sec/op    │   sec/op     vs base                 │    sec/op      vs base                 │    sec/op      vs base                 │    sec/op      vs base                │
atomic.Int64      62.51n ±  4%   63.89n ± 6%    +2.21% (p=0.035 n=10)    71.47n ±  6%   +14.34% (p=0.000 n=10)   208.60n ±  2%  +233.73% (p=0.000 n=10)   121.70n ±  1%  +94.70% (p=0.000 n=10)
atomic.Int64 2    47.34n ±  9%   65.33n ± 5%   +38.01% (p=0.001 n=10)    87.12n ± 12%   +84.06% (p=0.000 n=10)   154.20n ± 40%  +225.76% (p=0.000 n=10)    83.16n ± 11%  +75.68% (p=0.000 n=10)
atomic.Int64 4    53.90n ±  9%   67.12n ± 5%   +24.54% (p=0.000 n=10)   116.05n ±  6%  +115.33% (p=0.000 n=10)   114.90n ± 16%  +113.19% (p=0.000 n=10)    68.37n ± 15%  +26.85% (p=0.000 n=10)
atomic.Int64 8    44.26n ± 23%   64.47n ± 6%   +45.65% (p=0.000 n=10)   118.85n ±  6%  +168.53% (p=0.000 n=10)    71.26n ± 12%   +60.99% (p=0.000 n=10)    54.82n ±  9%  +23.86% (p=0.005 n=10)
atomic.Int64 16   43.37n ± 21%   67.89n ± 6%   +56.55% (p=0.000 n=10)   112.50n ±  2%  +159.40% (p=0.000 n=10)    52.10n ± 13%   +20.14% (p=0.000 n=10)    48.46n ±  2%        ~ (p=0.089 n=10)
atomic.Int64 32   36.38n ± 34%   67.99n ± 5%   +86.89% (p=0.000 n=10)   115.85n ±  2%  +218.44% (p=0.000 n=10)    49.17n ± 16%         ~ (p=0.075 n=10)    42.60n ± 24%        ~ (p=0.165 n=10)
atomic.Int64 64   26.04n ±  7%   68.11n ± 4%  +161.63% (p=0.000 n=10)    69.30n ±  3%  +166.18% (p=0.000 n=10)    28.97n ± 18%   +11.27% (p=0.001 n=10)    28.03n ± 12%        ~ (p=0.353 n=10)
geomean           43.40n         66.38n        +52.95%                   96.44n        +122.22%                   79.20n         +82.50%                   57.95n        +33.53%
```

Using mutex values:

```
$ benchstat -col /map -row "/value,/gomaxprocs" -filter /value:/Mutex/ map.bench
goos: linux
goarch: amd64
pkg: github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/research/tracking
cpu: Intel(R) Xeon(R) CPU @ 2.80GHz
              │ std+prealloc │             std+global              │              std+RWMutex              │                sync.Map                │              xsync.MapOf              │
              │    sec/op    │   sec/op     vs base                │    sec/op     vs base                 │    sec/op      vs base                 │    sec/op      vs base                │
MutexInt64      69.21n ±  3%   68.91n ± 5%        ~ (p=0.159 n=10)    71.36n ± 6%    +3.10% (p=0.003 n=10)   214.20n ±  1%  +209.49% (p=0.000 n=10)   128.10n ±  2%  +85.09% (p=0.000 n=10)
MutexInt64 2    67.98n ± 35%   72.66n ± 5%        ~ (p=0.617 n=10)   110.80n ± 4%   +62.98% (p=0.000 n=10)   154.45n ± 15%  +127.18% (p=0.000 n=10)    84.37n ±  5%  +24.10% (p=0.000 n=10)
MutexInt64 4    55.84n ± 30%   72.65n ± 6%  +30.12% (p=0.000 n=10)   132.10n ± 4%  +136.59% (p=0.000 n=10)   105.20n ± 22%   +88.41% (p=0.000 n=10)    79.50n ± 11%  +42.38% (p=0.000 n=10)
MutexInt64 8    54.16n ± 13%   72.68n ± 6%  +34.18% (p=0.000 n=10)   128.60n ± 3%  +137.42% (p=0.000 n=10)    79.45n ±  7%   +46.69% (p=0.000 n=10)    64.75n ±  4%  +19.53% (p=0.000 n=10)
MutexInt64 16   68.81n ±  5%   72.72n ± 6%   +5.68% (p=0.015 n=10)   124.10n ± 1%   +80.35% (p=0.000 n=10)    74.62n ± 12%         ~ (p=0.093 n=10)    72.50n ±  7%   +5.36% (p=0.019 n=10)
MutexInt64 32   90.62n ± 10%   69.91n ± 4%  -22.85% (p=0.000 n=10)   128.95n ± 2%   +42.30% (p=0.000 n=10)    87.30n ± 17%         ~ (p=0.796 n=10)    85.99n ±  8%        ~ (p=0.315 n=10)
MutexInt64 64   98.52n ± 19%   71.53n ± 8%  -27.40% (p=0.000 n=10)   137.65n ± 7%   +39.72% (p=0.000 n=10)    96.33n ± 18%         ~ (p=0.853 n=10)    92.24n ± 11%        ~ (p=0.631 n=10)
geomean         70.60n         71.57n        +1.37%                   116.8n        +65.38%                   108.2n         +53.25%                   84.98n        +20.36%
```

On systems with 8+ cores, `xsync.MapOf` shows the best performance.

## Value

```
$ benchstat -col '/value@(MutexInt64 atomic.Int64)' -row "/gomaxprocs" -filter /map:/xsync/ map.bench
goos: linux
goarch: amd64
pkg: github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/research/tracking
cpu: Intel(R) Xeon(R) CPU @ 2.80GHz
        │  MutexInt64  │             atomic.Int64             │
        │    sec/op    │    sec/op     vs base                │
          128.1n ±  2%   121.7n ±  1%   -5.00% (p=0.000 n=10)
2         84.37n ±  5%   83.16n ± 11%        ~ (p=0.256 n=10)
4         79.50n ± 11%   68.37n ± 15%  -14.01% (p=0.011 n=10)
8         64.75n ±  4%   54.82n ±  9%  -15.33% (p=0.000 n=10)
16        72.50n ±  7%   48.46n ±  2%  -33.16% (p=0.000 n=10)
32        85.99n ±  8%   42.60n ± 24%  -50.46% (p=0.000 n=10)
64        92.24n ± 11%   28.03n ± 12%  -69.61% (p=0.000 n=10)
geomean   84.98n         57.95n        -31.80%
```

A further optimization towards atomics appears reasonable too, given it scales with increasing core count, unlike mutexes.

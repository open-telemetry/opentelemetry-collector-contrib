# Testing the SQL Server Receiver

## 1. Unit tests

Run the receiver tests (including config load and per-event `collection_interval`):

```bash
cd receiver/sqlserverreceiver
go test ./... -v -count=1
```

To test only config loading with the per-event config:

```bash
go test ./... -v -run TestLoadConfig
```

## 2. Build the collector with the sqlserver receiver

The sqlserver receiver depends on the **core** scraperhelper (with `WithLogsSchedules`). This repo’s `receiver/sqlserverreceiver/go.mod` has a **replace** pointing at your local core. That replace is used when you run tests from `receiver/sqlserverreceiver`.

To build the **full** `otelcontribcol` binary that uses the same core code you have two options:

**Option A – Replace at the place you build from**

If you build from `opentelemetry-collector-contrib` root (e.g. `go build ./cmd/otelcontribcol` or `make otelcontribcol`), the root `go.mod` is used. Add the same replace there so the binary uses your local core scraperhelper:

```bash
# In opentelemetry-collector-contrib root, add to go.mod (adjust path if your layout differs):
replace go.opentelemetry.io/collector/scraper/scraperhelper => ../opentelemetry-collector/scraper/scraperhelper
```

Then from the contrib root:

```bash
go build -o otelcontribcol ./cmd/otelcontribcol
# or
make otelcontribcol
```

**Option B – Build only the receiver and run tests**

If you only need to validate the receiver logic and config, you can stay in the receiver directory and run tests; the replace in `receiver/sqlserverreceiver/go.mod` is enough:

```bash
cd receiver/sqlserverreceiver
go test ./... -v
```

## 3. Run with a test config (per-event intervals)

Use a config that turns on events with **different** `collection_interval` values so you can see them fire at different times.

**Option A: With a real SQL Server**

Create `config.yaml` (adjust `server`, `username`, `password`, `port`):

```yaml
receivers:
  sqlserver:
    collection_interval: 10s
    server: localhost
    username: sa
    password: YourPassword
    port: 1433
    top_query_collection:
      max_query_sample_count: 1000
      top_query_count: 200
    query_sample_collection:
      max_rows_per_query: 100
    events:
      db.server.top_query:
        collection_interval: 20s
        enabled: true
      db.server.query_sample:
        collection_interval: 8s
        enabled: true

exporters:
  debug:
    verbosity: basic

service:
  pipelines:
    logs:
      receivers: [sqlserver]
      exporters: [debug]
  telemetry:
    logs:
      level: debug
```

Then run:

```bash
./otelcontribcol --config config.yaml
```

**Option B: Without SQL Server (config-only check)**

To only verify the collector starts and the config is valid (logs pipeline will fail to connect, but config load is tested):

```yaml
receivers:
  sqlserver:
    collection_interval: 10s
    server: localhost
    username: sa
    password: test
    port: 1433
    events:
      db.server.top_query:
        collection_interval: 60s
        enabled: true
      db.server.query_sample:
        collection_interval: 15s
        enabled: true

exporters:
  debug:

service:
  pipelines:
    logs:
      receivers: [sqlserver]
      exporters: [debug]
```

Run:

```bash
./otelcontribcol --config config.yaml
```

If the config is valid and the receiver uses per-event schedules, the collector will start; you may see connection errors until a real SQL Server is available.

## 4. What to verify (per-event collection_interval)

1. **Config load**  
   No error about unknown keys. `events.db.server.top_query.collection_interval` and `events.db.server.query_sample.collection_interval` are accepted.

2. **Different intervals**  
   With a real SQL Server and `debug` exporter:
   - Use e.g. `db.server.query_sample.collection_interval: 8s` and `db.server.top_query.collection_interval: 20s`.
   - In debug logs, you should see log records for `db.server.query_sample` more often than for `db.server.top_query` (e.g. roughly every 8s vs every 20s).

3. **Unit test**  
   `TestLoadConfig/named` in `config_test.go` loads `testdata/config.yaml` and asserts that:
   - `Events.DbServerQuerySample.CollectionInterval == 15s`
   - `Events.DbServerTopQuery.CollectionInterval == 80s`

## 5. Quick config-only test (no build)

Validate that the receiver’s config structure is correct and that the test data config loads:

```bash
cd receiver/sqlserverreceiver
go test . -run TestLoadConfig -v
```

This confirms that per-event `collection_interval` is parsed and used as intended.

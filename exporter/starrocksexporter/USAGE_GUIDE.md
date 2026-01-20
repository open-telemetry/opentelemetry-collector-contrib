# StarRocks Exporter Usage Guide

## How to Pass StarRocks Server Configuration

There are 3 main ways to pass configuration when running OpenTelemetry Collector:

---

## 1. Using Config File (YAML) - Recommended

### Step 1: Create a YAML config file

Create a file `otel-collector-config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    send_batch_size: 10000
    timeout: 5s
  memory_limiter:
    check_interval: 2s
    limit_mib: 1800

exporters:
  starrocks:
    # ===== REQUIRED CONFIG =====
    endpoint: localhost:9030        # StarRocks MySQL endpoint
    username: root                   # Username for connection
    password: your_password         # Password
    database: otel                  # Database name
    
    # ===== OPTIONAL CONFIG =====
    # Table names
    logs_table_name: otel_logs
    traces_table_name: otel_traces
    
    # Schema management
    create_schema: true             # Automatically create database and tables
    
    # Connection pool
    max_open_conns: 20
    max_idle_conns: 10
    conn_max_lifetime: 10m
    
    # Connection parameters (optional)
    connection_params:
      timeout: "10s"
      readTimeout: "30s"
    
    # Metrics tables
    metrics_tables:
      gauge:
        name: "otel_metrics_gauge"
      sum:
        name: "otel_metrics_sum"
      summary:
        name: "otel_metrics_summary"
      histogram:
        name: "otel_metrics_histogram"
      exponential_histogram:
        name: "otel_metrics_exponential_histogram"
    
    # Timeout
    timeout: 5s
    
    # Retry configuration
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s
    
    # Queue configuration
    sending_queue:
      enabled: true
      num_consumers: 10
      queue_size: 1000
    
    # TLS (if needed)
    # tls:
    #   cert_file: /path/to/client.crt
    #   key_file: /path/to/client.key
    #   ca_file: /path/to/ca.crt
    #   insecure: false

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [starrocks]
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [starrocks]
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [starrocks]
```

### Step 2: Run Collector with config file

```bash
# Method 1: Using --config flag
./otelcontribcol --config=otel-collector-config.yaml

# Method 2: Using environment variable
export OTEL_CONFIG_FILE=otel-collector-config.yaml
./otelcontribcol

# Method 3: Using Docker
docker run -v $(pwd)/otel-collector-config.yaml:/etc/otel-collector-config.yaml \
  otelcontribcol:latest \
  --config=/etc/otel-collector-config.yaml
```

---

## 2. Using Environment Variables

OpenTelemetry Collector supports overriding config with environment variables:

```yaml
exporters:
  starrocks:
    endpoint: ${STARROCKS_ENDPOINT}
    username: ${STARROCKS_USERNAME}
    password: ${STARROCKS_PASSWORD}
    database: ${STARROCKS_DATABASE}
```

### Set environment variables:

```bash
export STARROCKS_ENDPOINT=localhost:9030
export STARROCKS_USERNAME=root
export STARROCKS_PASSWORD=your_password
export STARROCKS_DATABASE=otel

./otelcontribcol --config=otel-collector-config.yaml
```

### Or inline:

```bash
STARROCKS_ENDPOINT=localhost:9030 \
STARROCKS_USERNAME=root \
STARROCKS_PASSWORD=your_password \
STARROCKS_DATABASE=otel \
./otelcontribcol --config=otel-collector-config.yaml
```

---

## 3. Using Command Line Flags (--set)

OpenTelemetry Collector supports overriding config with `--set` flag:

```bash
./otelcontribcol \
  --config=otel-collector-config.yaml \
  --set=exporters.starrocks.endpoint=localhost:9030 \
  --set=exporters.starrocks.username=root \
  --set=exporters.starrocks.password=your_password \
  --set=exporters.starrocks.database=otel
```

---

## Real-World Examples

### Example 1: Minimal Config

```yaml
exporters:
  starrocks:
    endpoint: localhost:9030
    username: root
    password: password
    database: otel

service:
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [starrocks]
```

### Example 2: Full Config with Production Settings

```yaml
exporters:
  starrocks:
    endpoint: starrocks.example.com:9030
    username: otel_user
    password: secure_password_123
    database: telemetry_prod
    
    # Disable auto-create schema (use existing schema)
    create_schema: false
    
    # Connection pool for high throughput
    max_open_conns: 50
    max_idle_conns: 20
    conn_max_lifetime: 30m
    
    # Custom table names
    logs_table_name: prod_logs
    traces_table_name: prod_traces
    
    # Metrics tables
    metrics_tables:
      gauge:
        name: "prod_metrics_gauge"
      sum:
        name: "prod_metrics_sum"
    
    # Timeout and retry
    timeout: 10s
    retry_on_failure:
      enabled: true
      initial_interval: 10s
      max_interval: 60s
      max_elapsed_time: 600s
    
    # Queue settings
    sending_queue:
      enabled: true
      num_consumers: 20
      queue_size: 10000
    
    # TLS
    tls:
      ca_file: /etc/ssl/ca.crt
      cert_file: /etc/ssl/client.crt
      key_file: /etc/ssl/client.key
      insecure: false
```

### Example 3: Docker Compose

Create `docker-compose.yml`:

```yaml
version: "3.8"

services:
  otel-collector:
    image: otelcontribcol:latest
    command: ["--config=/etc/otel-collector-config.yaml"]
    volumes:
      - ./otel-collector-config.yaml:/etc/otel-collector-config.yaml
    ports:
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    environment:
      - STARROCKS_ENDPOINT=starrocks:9030
      - STARROCKS_USERNAME=root
      - STARROCKS_PASSWORD=password
    depends_on:
      - starrocks

  starrocks:
    image: starrocks/fe-ubuntu:latest
    ports:
      - "9030:9030"
      - "8030:8030"
    environment:
      - MYSQL_ROOT_PASSWORD=password
```

Run:
```bash
docker-compose up -d
```

---

## Ways to Run Collector

### 1. Direct Binary

```bash
# Build collector with starrocks exporter
make otelcontribcol

# Run
./bin/otelcontribcol_darwin_arm64 --config=config.yaml
```

### 2. Docker

```bash
# Build image
docker build -t otelcontribcol:latest .

# Run
docker run -v $(pwd)/config.yaml:/etc/config.yaml \
  otelcontribcol:latest \
  --config=/etc/config.yaml
```

### 3. Kubernetes

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: otel-collector-config
data:
  config.yaml: |
    exporters:
      starrocks:
        endpoint: starrocks-service:9030
        username: root
        password: ${STARROCKS_PASSWORD}
        database: otel
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otel-collector
spec:
  template:
    spec:
      containers:
      - name: otel-collector
        image: otelcontribcol:latest
        args: ["--config=/etc/config.yaml"]
        volumeMounts:
        - name: config
          mountPath: /etc/config.yaml
          subPath: config.yaml
        env:
        - name: STARROCKS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: starrocks-secret
              key: password
      volumes:
      - name: config
        configMap:
          name: otel-collector-config
```

---

## Verify Config

### Check if config is valid:

```bash
./otelcontribcol --config=config.yaml --dry-run
```

### Test connection:

```bash
# Test MySQL connection to StarRocks
mysql -h localhost -P 9030 -u root -p
```

---

## Troubleshooting

### Error: "endpoint must be specified"

**Cause**: Missing `endpoint` in config

**Solution**: Add `endpoint: host:port` to config

### Error: "failed to ping database"

**Cause**: 
- StarRocks server not running
- Wrong endpoint
- Wrong username/password
- Network connectivity issues

**Solution**:
1. Check StarRocks is running: `mysql -h host -P 9030 -u root -p`
2. Verify endpoint format: `host:port` (no `tcp://` or `mysql://` prefix)
3. Check username/password
4. Check firewall/network

### Error: "table doesn't exist"

**Cause**: `create_schema: false` but tables not created

**Solution**: 
- Set `create_schema: true` to auto-create
- Or create tables manually using DDL in `internal/sqltemplates`

---

## Best Practices

1. ✅ **Production**: Set `create_schema: false` and manage schema manually
2. ✅ **Development**: Can use `create_schema: true` for auto-creation
3. ✅ **Security**: Use environment variables or secrets for password
4. ✅ **Performance**: Tune `max_open_conns` and `queue_size` based on workload
5. ✅ **Monitoring**: Monitor connection pool and queue metrics

---

## Quick Start

```bash
# 1. Create config file
cat > config.yaml <<EOF
exporters:
  starrocks:
    endpoint: localhost:9030
    username: root
    password: password
    database: otel

service:
  pipelines:
    logs:
      receivers: [otlp]
      exporters: [starrocks]
EOF

# 2. Run collector
./otelcontribcol --config=config.yaml
```

---

Happy using! 🚀

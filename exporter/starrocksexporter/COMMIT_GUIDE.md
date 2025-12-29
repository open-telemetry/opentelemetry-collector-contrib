# Hướng dẫn Commit và Push StarRocks Exporter

## Files đã tạo/sửa

### Files mới (exporter/starrocksexporter/):
- ✅ `factory.go` - Exporter factory
- ✅ `config.go` - Configuration struct và validation
- ✅ `exporter_logs.go` - Logs exporter implementation
- ✅ `exporter_traces.go` - Traces exporter implementation
- ✅ `exporter_metrics.go` - Metrics exporter implementation
- ✅ `factory_test.go` - Factory tests
- ✅ `config_test.go` - Config tests
- ✅ `generated_component_test.go` - Component lifecycle tests
- ✅ `testdata/config.yaml` - Test configuration
- ✅ `go.mod` - Dependencies
- ✅ `go.sum` - Dependencies checksums
- ✅ `README.md` - Documentation
- ✅ `metadata.yaml` - Component metadata
- ✅ `internal/` - Internal utilities và SQL templates

### Files đã sửa:
- ✅ `cmd/otelcontribcol/builder-config.yaml` - Đã thêm starrocks exporter

---

## Các bước để Commit và Push

### 1. Tạo branch mới (khuyến nghị)

```bash
# Từ root của repo
git checkout -b add-starrocks-exporter
```

Hoặc nếu muốn dùng tên khác:
```bash
git checkout -b exporter/starrocks
```

---

### 2. Stage các files

```bash
# Stage exporter directory
git add exporter/starrocksexporter/

# Stage builder config
git add cmd/otelcontribcol/builder-config.yaml

# Verify những gì sẽ commit
git status
```

---

### 3. Commit với message phù hợp

```bash
git commit -m "exporter/starrocks: Add StarRocks exporter

This commit adds a new StarRocks exporter that supports sending
OpenTelemetry data (logs, traces, metrics) to StarRocks database
using MySQL protocol.

Features:
- Support for logs, traces, and metrics export
- MySQL protocol connection (port 9030)
- Automatic schema creation
- Connection pool configuration
- Configurable table names
- TLS support (config struct ready)
- Comprehensive unit tests

The exporter follows the same patterns as ClickHouse exporter
but uses MySQL driver instead of ClickHouse native driver.

Tests:
- All unit tests pass
- Config validation tests
- Factory creation tests
- Component lifecycle tests
- YAML config loading tests"
```

Hoặc message ngắn gọn hơn:
```bash
git commit -m "exporter/starrocks: Add StarRocks exporter

Add new exporter for StarRocks database using MySQL protocol.
Supports logs, traces, and metrics with automatic schema creation.
Includes comprehensive unit tests."
```

---

### 4. Push lên remote

```bash
# Push branch lên origin
git push origin add-starrocks-exporter

# Hoặc nếu branch name khác
git push origin exporter/starrocks
```

---

### 5. Tạo Pull Request

Sau khi push, tạo PR trên GitHub:
1. Vào repository: https://github.com/DucHungGithub/opentelemetry-collector-contrib
2. Click "Compare & pull request"
3. Điền PR description
4. Submit PR

---

## PR Description Template

```markdown
## Description

This PR adds a new StarRocks exporter for OpenTelemetry Collector Contrib.

### Features
- ✅ Export logs, traces, and metrics to StarRocks
- ✅ MySQL protocol support (port 9030)
- ✅ Automatic database and table creation
- ✅ Connection pool configuration
- ✅ Configurable table names
- ✅ Comprehensive unit tests

### Implementation Details
- Uses `go-sql-driver/mysql` for MySQL protocol connection
- Follows same patterns as ClickHouse exporter
- Includes comprehensive test coverage
- Registered in builder-config.yaml

### Testing
- ✅ All unit tests pass
- ✅ Config validation tests
- ✅ Factory creation tests
- ✅ Component lifecycle tests
- ✅ YAML config loading tests

### Documentation
- ✅ README.md with examples
- ✅ Configuration options documented
- ✅ Test data examples

## Type of Change
- [x] New feature (non-breaking change which adds functionality)

## Checklist
- [x] Code follows the project's style guidelines
- [x] Self-review completed
- [x] Comments added for complex code
- [x] Documentation updated
- [x] Tests added/updated
- [x] All tests pass
```

---

## Quick Commands (Copy & Paste)

```bash
# 1. Tạo branch
git checkout -b add-starrocks-exporter

# 2. Stage files
git add exporter/starrocksexporter/ cmd/otelcontribcol/builder-config.yaml

# 3. Commit
git commit -m "exporter/starrocks: Add StarRocks exporter

Add new exporter for StarRocks database using MySQL protocol.
Supports logs, traces, and metrics with automatic schema creation.
Includes comprehensive unit tests."

# 4. Push
git push origin add-starrocks-exporter
```

---

## Verify trước khi commit

### 1. Kiểm tra tests
```bash
cd exporter/starrocksexporter
go test ./...
```

### 2. Kiểm tra linter (nếu có)
```bash
golangci-lint run exporter/starrocksexporter
```

### 3. Kiểm tra files sẽ commit
```bash
git status
git diff --cached  # Xem staged changes
```

---

## Lưu ý

1. ✅ **Đảm bảo tests pass** trước khi commit
2. ✅ **Không commit** các file tạm hoặc không cần thiết
3. ✅ **Commit message** nên rõ ràng, mô tả đúng changes
4. ✅ **Tạo branch mới** thay vì commit trực tiếp vào main
5. ✅ **PR description** nên đầy đủ để reviewers hiểu rõ

---

## Files không nên commit

- ❌ `*.swp`, `*.swo` (vim temp files)
- ❌ `.DS_Store` (macOS)
- ❌ `*.log` files
- ❌ Personal notes/checklists (nếu có)

---

## Sau khi PR được merge

1. Sync với upstream:
   ```bash
   git checkout main
   git pull upstream main
   ```

2. Delete local branch:
   ```bash
   git branch -d add-starrocks-exporter
   ```

---

## Troubleshooting

### Nếu push bị reject:
```bash
# Pull latest changes trước
git pull origin main --rebase

# Resolve conflicts nếu có
# Sau đó push lại
git push origin add-starrocks-exporter
```

### Nếu quên add file:
```bash
git add <file>
git commit --amend --no-edit
git push origin add-starrocks-exporter --force
```

---

Chúc bạn contribute thành công! 🎉


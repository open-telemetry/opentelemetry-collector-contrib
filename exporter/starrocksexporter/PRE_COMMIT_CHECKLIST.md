# Pre-Commit Checklist for StarRocks Exporter

## ✅ Checks to Run Before PR Approval

### 1. Format Code
```bash
cd exporter/starrocksexporter
gofmt -w .
```

### 2. Run Unit Tests
```bash
cd exporter/starrocksexporter
go test ./... -v
```

### 3. Run Go Vet
```bash
cd exporter/starrocksexporter
go vet ./...
```

### 4. Validate Changelog
```bash
# From repo root
make chlog-validate
```

### 5. Check Module Versions
```bash
# From repo root
make genotelcontribcol
# Then check if versions are aligned
```

### 6. Lint Check
```bash
# From repo root
make golint
# Or for specific component
cd exporter/starrocksexporter
make lint
```

### 7. Build Check
```bash
# From repo root
make otelcontribcol
```

## Current Status

- ✅ Code formatted (gofmt)
- ✅ Unit tests written
- ✅ Changelog entry created
- ✅ CODEOWNERS updated
- ✅ Builder config updated
- ⚠️ Need to verify: module versions alignment

## Known Issues

1. **Sandbox Restrictions**: Some tests may fail in sandbox due to permission issues. Run tests outside sandbox if needed.

2. **Module Versions**: Ensure all `go.opentelemetry.io/collector` module versions match between:
   - `exporter/starrocksexporter/go.mod`
   - `exporter/clickhouseexporter/go.mod` (reference)
   - Root `go.mod` (if applicable)

## After Maintainer Approval

Once workflows are approved and running, monitor:
- ✅ All checks pass
- ✅ No lint errors
- ✅ No test failures
- ✅ Module version checks pass
- ✅ Changelog validation passes


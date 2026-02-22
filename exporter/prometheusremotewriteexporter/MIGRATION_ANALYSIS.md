# Backward Compatibility Analysis: ClientConfig Nesting Migration

## Executive Summary

**Yes, removing squash would break existing configurations.** However, a safe migration path exists using the deprecation pattern established by other OTel exporters (e.g., elasticsearchexporter). This approach allows both old and new config formats to work during a deprecation period.

---

## 1. Would Removing Squash Break Existing User Configurations?

### ✅ YES - BREAKING CHANGE WITHOUT MITIGATION

**Evidence:**
- Testdata explicitly uses flat HTTP client fields at root level:
  - `endpoint: "localhost:8888"`
  - `tls: { ca_file: ... }`
  - `write_buffer_size: 524288`
  - `headers: { ... }`
  - `compression: "gzip"`

- README documents these fields at the root level:
  - "endpoint (no default): The remote write URL..."
  - "headers: additional headers attached to each HTTP request."
  - "TLS and mTLS settings" under "Advanced Configuration"

- Thousands of real-world deployments likely use flat configuration like:
  ```yaml
  exporters:
    prometheusremotewrite:
      endpoint: "https://cortex:9411"
      compression: snappy
      tls:
        insecure: false
  ```

**Impact if not mitigated:**
- Configs stop being accepted without any helpful error message
- Users must restructure their entire configuration
- Silent failures are unlikely—mapstructure would likely ignore unknown nested fields

---

## 2. Does config.schema.yaml Expose HTTP Timeout or Other Client Fields at Root Level?

### ✅ YES - EXTENSIVELY

**Current schema structure (`config.schema.yaml` lines 75-77):**
```yaml
allOf:
  - $ref: go.opentelemetry.io/collector/exporter/exporterhelper.timeout_config
  - $ref: go.opentelemetry.io/collector/config/configretry.back_off_config
  - $ref: go.opentelemetry.io/collector/config/confighttp.client_config  # ← Squashed at root!
```

**All these fields are exposed at root level (will break if removed):**
- `endpoint` (Endpoint)
- `timeout` (Timeout - HTTP client timeout)
- `compression` (Compression)
- `tls` (TLS config)
- `headers` (Headers)
- `write_buffer_size` (WriteBufferSize)
- `read_buffer_size` (ReadBufferSize)
- `auth.*` (Auth settings)
- `proxy_url` (ProxyURL)
- `http_version` (HTTPVersion)
- Many others from `confighttp.ClientConfig`

**Risk:** Users relying on any of these at the root level will see configuration failures or silent ignoring.

---

## 3. Do Tests and Testdata Rely on Flat Timeout Behavior?

### ✅ YES - MULTIPLE INSTANCES

**Direct test configuration (`config_test.go` line 44):**
```go
clientConfig.Timeout = 5 * time.Second
```
This is then embedded in test expectations that use flat structure.

**Test data (`testdata/config.yaml`):**
- Uses all flat client config fields
- Tests validate that these root-level fields work correctly
- No current tests validate nested `client.*` access

**Code relying on flat structure (`factory.go` line 108):**
```go
clientConfig.Timeout = exporterhelper.NewDefaultTimeoutConfig().Timeout
```
This manually works around the collision by forcing both timeouts to be identical.

---

## 4. Would Existing Configs Silently Stop Working or Fail Validation?

### ⚠️ BEHAVIOR DEPENDS ON IMPLEMENTATION

**Scenario A: Naïve removal (squash removed, no compatibility layer)**
- **Result:** Configuration accepted but values silently ignored
- **User experience:** No error → app starts → no data sent or wrong timeout used
- **Severity:** CRITICAL - Silent failure is dangerous in production

**Scenario B: Config file with known fields at root level**
```yaml
exporters:
  prometheusremotewrite:
    endpoint: "https://cortex:9411"      # ← Would be unknown field
    compression: snappy                   # ← Would be unknown field
```
- mapstructure would log warning about unknown fields (if configured to do so)
- Config loads but fields ignored
- **Critical operational issue:** HTTP client not configured correctly

**Scenario C: With compatibility layer (recommended)**
- Old format works with deprecation warning
- New format works without warning
- Smooth migration path for users

---

## 5. Should We Implement a Transitional Compatibility Layer?

### ✅ YES - STRONGLY RECOMMENDED

**Precedent:** elasticsearchexporter uses this exact pattern with `Unmarshal` override.

**Implementation approach:**

1. **Add nested ClientConfig field:**
   ```go
   type Config struct {
       // ... existing fields ...
       
       // Keep for backward compatibility (will be deprecated)
       TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"`
       configretry.BackOffConfig `mapstructure:"retry_on_failure"`
       
       // NEW: Nested HTTP client config (NOT squashed)
       ClientConfig confighttp.ClientConfig `mapstructure:"client"`
       
       // Keep old for backward compat
       _ClientConfigLegacy confighttp.ClientConfig `mapstructure:",squash"`
   }
   ```

2. **Override Unmarshal to handle migration:**
   ```go
   func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
       if err := conf.Unmarshal(cfg); err != nil {
           return err
       }
       
       // If client.*-style config is not set but root-level HTTP fields ARE set,
       // assume legacy format and copy to new location
       if !conf.IsSet("client") && conf.IsSet("endpoint") {
           cfg.ClientConfig = cfg._ClientConfigLegacy
       }
       
       // If timeout collision: prefer exporter timeout, apply to HTTP too
       if cfg.ClientConfig.Timeout == 0 {
           cfg.ClientConfig.Timeout = cfg.TimeoutSettings.Timeout
       }
       
       return nil
   }
   ```

3. **Add deprecation warning:**
   - Call `handleDeprecatedConfig()` in Validate
   - Warn when root-level HTTP fields are detected
   - Point users to new `client.*` format

**Example warning:**
```
The flat HTTP client configuration (endpoint, tls, headers, etc.) is deprecated 
and will be removed in v0.X.X. Please migrate to the nested format:
    
  Old:
    endpoint: "https://cortex:9411"
    tls:
      insecure: false
      
  New:
    client:
      endpoint: "https://cortex:9411"
      tls:
        insecure: false
```

---

## 6. Would This Require a Deprecation Notice or Changelog Entry?

### ✅ YES - REQUIRED FOR PROPER MAINTENANCE

**Recommended changelog entry (CHANGELOG.md):**
```markdown
### Changed
- `prometheusremotewrite` exporter: HTTP client configuration now uses nested `client:` field 
  instead of flat root-level fields (endpoint, compression, headers, tls, etc.). 
  The old flat format is deprecated and will be removed in [version X.X.X]. 
  see [MIGRATION_GUIDE.md](exporter/prometheusremotewriteexporter/MIGRATION_GUIDE.md) for details.
```

**What needs documentation:**
1. **Migration guide** - Show old vs new format with examples
2. **Changelog entry** - Version you're deprecating in, removal timeline
3. **README update** - Move HTTP config under "Advanced Configuration" → nested structure
4. **In-code comments** - Mark squashed fields as deprecated
5. **Deprecation warning** - Log helpful message to users when old format detected

**Recommended removal timeline:**
- Current version: Deprecation warning only (both formats work)
- +2 versions: Remove squashed fields, require nested format
- This gives users ~3-6 months to migrate

---

## Risk Assessment Matrix

| Aspect | Risk Level | Mitigation |
|--------|-----------|-----------|
| User config breakage | 🔴 CRITICAL | Implement Unmarshal compatibility layer |
| Silent failures | 🔴 CRITICAL | Validate and warn when flat format detected |
| Documentation gaps | 🟡 MEDIUM | Create migration guide + update README |
| Testing coverage | 🟡 MEDIUM | Add tests for both old and new formats |
| Adoption friction | 🟡 MEDIUM | Clear deprecation messaging + documentation |
| Schema inconsistency | 🟢 LOW | Schema auto-generates from struct tags |

---

## Recommended Implementation Approach

### Phase 1: Deprecation (Target your next release)
1. ✅ Add new nested `client` field to Config struct
2. ✅ Implement `Unmarshal()` override to handle migration
3. ✅ Keep squashed fields (will be deprecated)
4. ✅ Add deprecation warnings when flat format detected
5. ✅ Update README with migration guide
6. ✅ Add tests for both old and new formats
7. ✅ Update CHANGELOG.md
8. ✅ Mark fields with deprecation comments

### Phase 2: Removal (2+ versions later)
1. Remove `mapstructure:",squash"` from squashed fields
2. Remove `Unmarshal()` override (migration layer)
3. Remove deprecation warnings
4. Update schema to show nested structure
5. Final README cleanup

---

## Recommended Validation Approach

```go
func (cfg *Config) Validate() error {
    handleDeprecatedConfig(cfg)  // Logs warnings
    
    // Existing validation...
    if cfg.MaxBatchRequestParallelism != nil && *cfg.MaxBatchRequestParallelism < 1 {
        return errors.New("max_batch_request_parallelism can't be set to below 1")
    }
    
    // ... rest of validation
    return nil
}

func handleDeprecatedConfig(cfg *Config) {
    // Check if using flat format (legacy)
    if cfg._ClientConfigLegacy.Endpoint != "" {
        logger.Warn(
            "Flat HTTP client configuration is deprecated. " +
            "Please migrate to nested format under 'client:'. " +
            "This will be removed in v0.X.X. " +
            "See migration guide at: " +
            "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/" +
            "exporter/prometheusremotewriteexporter/MIGRATION_GUIDE.md",
        )
    }
}
```

---

## Summary

| Question | Answer | Severity |
|----------|--------|----------|
| Would removing squash break configs? | YES | 🔴 CRITICAL |
| Are HTTP fields exposed at root? | YES | 🔴 CRITICAL |
| Do tests rely on flat format? | YES | 🟡 MEDIUM |
| Would configs silently fail? | MAYBE (w/o mitigation) | 🔴 CRITICAL |
| Need compatibility layer? | YES (STRONGLY) | 🔴 CRITICAL |
| Need deprecation period? | YES | 🟡 MEDIUM |

**Conclusion:** Implement the deprecation pattern used by elasticsearchexporter. This provides a smooth 2-phase migration: first with warnings while both formats work, then removal of legacy format after suitable deprecation period.

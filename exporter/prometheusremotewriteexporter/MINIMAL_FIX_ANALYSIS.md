# Minimal Fix Analysis: Timeout Collision Without Schema Redesign

## Problem Statement

**Current issue:** Both `TimeoutSettings` and `ClientConfig` are squashed at the root level, causing their `Timeout` fields to collide during YAML decoding. Users cannot configure exporter timeout and HTTP client timeout independently.

**Constraint:** Fix ONLY the timeout collision issue without doing a full HTTP config nesting/schema redesign. Keep all HTTP client fields at root level.

---

## Option 1: Custom Unmarshal with Explicit Timeout Separation

### How it works:
1. Keep both structs squashed (no schema change)
2. Add a new explicit field `ExporterTimeout time.Duration` at root level
3. Override `Unmarshal()` to detect which timeout was set and apply it correctly
4. Document that users should use `exporter_timeout` for exporter timeout, standard `timeout` for HTTP client

### Implementation:
```go
type Config struct {
    TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"` // Timeout field conflicts
    configretry.BackOffConfig `mapstructure:"retry_on_failure"`
    
    // NEW: Explicit field to avoid collision
    ExporterTimeout time.Duration `mapstructure:"exporter_timeout"`
    
    // ... rest of fields ...
    ClientConfig confighttp.ClientConfig `mapstructure:",squash"` // Exposes Timeout field
}

func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
    if err := conf.Unmarshal(cfg); err != nil {
        return err
    }
    
    // Handle timeout collision: ExporterTimeout takes precedence over squashed TimeoutSettings.Timeout
    if cfg.ExporterTimeout > 0 {
        cfg.TimeoutSettings.Timeout = cfg.ExporterTimeout
    }
    
    return nil
}
```

### Config usage (new way):
```yaml
exporters:
  prometheusremotewrite:
    endpoint: "https://cortex:9411"
    timeout: 5s              # HTTP client timeout (from ClientConfig)
    exporter_timeout: 10s    # Exporter timeout (from ExporterTimeout)
```

### Pros:
- ✅ Zero schema changes (HTTP fields stay at root)
- ✅ Backward compatible (old configs still work, but with merged timeout)
- ✅ Minimal code changes (just add one field + simple Unmarshal)
- ✅ Clear separation: `timeout` = HTTP, `exporter_timeout` = exporter
- ✅ Follows Go naming conventions

### Cons:
- ❌ Two separate timeout fields is less clean than one "real" separation
- ❌ Users need to document/understand which timeout to use
- ❌ Slightly confusing: `timeout` vs `exporter_timeout` naming

### Backward Compatibility:
- ✅ Existing configs with single `timeout: 5s` continue to work → applies to both
- ✅ New configs can use both `timeout:` and `exporter_timeout:`
- Migration: None needed, works immediately

### Risks:
- 🟢 LOW: Single new field is minimal and non-invasive
- 🟢 LOW: Simple override logic, easy to test

### Complexity: **1/10** (Minimal)

---

## Option 2: Remove mapstructure Squash from TimeoutSettings Only

### How it works:
1. Remove `mapstructure:",squash"` from TimeoutSettings
2. Give it an explicit root-level field name: `timeout:` (or keep the embedded struct name)
3. Keep ClientConfig squashed
4. Result: Two different timeout keys at root level

### Implementation:
```go
type Config struct {
    // Remove squash, give explicit name
    TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:"timeout"`
    
    configretry.BackOffConfig `mapstructure:"retry_on_failure"`
    // ... rest of fields ...
    ClientConfig confighttp.ClientConfig `mapstructure:",squash"`
}
```

### Config usage (new way):
```yaml
exporters:
  prometheusremotewrite:
    endpoint: "https://cortex:9411"
    timeout:           # Exporter timeout (from TimeoutSettings)
      timeout: 10s
    compression: snappy        # Still at root (from ClientConfig)
    write_buffer_size: 512000  # Still at root (from ClientConfig)
```

### Pros:
- ✅ True separation: `timeout.*` vs `timeout` (for HTTP) — wait, this doesn't work!
- ✅ Explicit schema (some clarity)

### Cons:
- ❌ **STILL COLLIDES!** TimeoutSettings fields would live under a `timeout:` key, but ClientConfig's squashed `Timeout` field also creates a root-level `timeout`. Still a collision!
- ❌ Breaks backward compatibility (existing `timeout: 5s` configs break)
- ❌ Ugly nesting required: `timeout: { timeout: 10s }` (confusing)
- ❌ HTTP config items stay at root, creating the exact schema inconsistency this is trying to solve

### Issues:
- 🔴 BROKEN: Doesn't actually solve the collision problem
- 🔴 BREAKING: Existing configs fail
- 🔴 UGLY: Would require nested structure

### Complexity: **8/10** (Complex + doesn't solve the issue)

---

## Option 3: Custom Decoding with Heuristic Logic

### How it works:
1. Keep both squashed (no schema change)
2. Override `Unmarshal()` with smart heuristic:
   - If user sets `timeout:` once, apply it to BOTH timeouts (current behavior)
   - If user specifies via some other mechanism (env var + file), let HTTP take precedence
   - Document: "For independent timeouts, use environment variable overrides"

### Implementation:
```go
func (cfg *Config) Unmarshal(conf *confmap.Conf) error {
    if err := conf.Unmarshal(cfg); err != nil {
        return err
    }
    
    // Heuristic: if only one timeout specified, use it for both
    // Users wanting independent timeouts must use env vars
    if cfg.ClientConfig.Timeout > 0 && cfg.TimeoutSettings.Timeout == 0 {
        cfg.TimeoutSettings.Timeout = cfg.ClientConfig.Timeout
    }
    
    return nil
}
```

### Config usage:
```yaml
exporters:
  prometheusremotewrite:
    endpoint: "https://cortex:9411"
    timeout: 5s  # Sets both timeouts
```

For different timeouts, users do:
```bash
# Override exporter timeout via env
export OTEL_EXPORTER_OTLP_TIMEOUT=10s
```

### Pros:
- ✅ Zero schema changes
- ✅ Backward compatible
- ✅ Minimal code changes

### Cons:
- ❌ Users CANNOT independently configure timeouts in YAML at all
- ❌ Relies on environment variable workaround (fragile, hard to document)
- ❌ Doesn't actually solve the core issue
- ❌ Confusing user experience

### Backward Compatibility:
- ✅ Works but doesn't truly enable independent config

### Complexity: **2/10** (Very simple, but doesn't solve the problem)

---

## Option 4: Rename TimeoutSettings Field in Struct

### How it works:
1. Rename the embedded `TimeoutSettings exporterhelper.TimeoutConfig` field to something that won't conflict
2. Keep ClientConfig squashed
3. Example: `ExporterTimeoutConfig exporterhelper.TimeoutConfig`

### Implementation:
```go
type Config struct {
    ExporterTimeoutConfig     exporterhelper.TimeoutConfig `mapstructure:",squash"` // Also has .Timeout
    // ^^^^ renamed but STILL squashed, STILL conflicts with ClientConfig.Timeout!
    
    ClientConfig confighttp.ClientConfig `mapstructure:",squash"`
}
```

### Issues:
- 🔴 **DOESN'T WORK**: Renaming the variable doesn't change what fields it exposes when squashed
- Both structs' fields still collide under the same names when squashed

### Complexity: **10/10** (Won't work)

---

## Option 5: Structured Timeout with Custom Marshaling

### How it works:
1. Keep `ClientConfig` squashed at root level (no schema change for HTTP)
2. Wrap `TimeoutSettings` in a custom type that handles marshaling separately
3. Create a custom marshalers that explicitly handle each timeout

### Complexity:
- 🔴 VERY complex: Requires custom MarshalText/UnmarshalText on TimeoutSettings
- 🔴 OTel ecosystem expects  standard types (exporterhelper.TimeoutConfig)
- 🔴 Would break integration with standard OTel modules

### Complexity: **9/10** (Over-engineered, breaks compatibility)

---

## Comparison Table

| Aspect | Option 1 (ExporterTimeout) | Option 2 (Remove Squash) | Option 3 (Heuristic) | Option 4 (Rename) | Option 5 (Custom) |
|--------|---------------------------|------------------------|----------------------|-------------------|------------------|
| **Solves timeout collision?** | ✅ YES | ❌ NO (still collides) | ❌ NO (not independent) | ❌ NO (field name irrelevant) | ✅ YES (overkill) |
| **Schema changes?** | 🟢 Minimal (+1 field) | 🔴 YES (breaks compat) | 🟢 None | 🟢 None | 🔴 Complex |
| **Backward compatible?** | ✅ YES | ❌ NO | ✅ YES | ✅ YES | ❓ Questionable |
| **Independent config?** | ✅ YES (via new field) | ✅ YES (but breaks old) | ❌ NO | N/A | ✅ YES |
| **Code complexity** | 🟢 Low (1 field + Unmarshal) | 🟡 Medium (breaks compat) | 🟢 Very low | 🟢 None | 🔴 High |
| **User clarity** | 🟡 Good (2 fields) | 🟡 Moderate | 🔴 Poor (env required) | N/A | 🟡 Complex |
| **Test coverage needed** | 🟡 Medium | 🔴 High (breaking) | 🟢 Low | 🟢 None | 🔴 High |
| **Documentation effort** | 🟡 Medium | 🔴 High | 🟡 Medium | 🟢 Low | 🔴 High |
| **Long-term maintainability** | ✅ Best | ❌ Bad | 🟡 OK | N/A | 🔴 Hard |

---

## Recommendation: **Option 1 - Add ExporterTimeout Field**

### Why it wins:

1. **Minimal schema impact:** Only adds one new field (exporter_timeout), no breaking changes
2. **Actually solves the problem:** Users can truly configure both timeouts independently
3. **Backward compatible:** Existing single-timeout configs continue to work
4. **Clean implementation:** Simple custom Unmarshal, straightforward logic
5. **Easy to document:** "Use `timeout` for HTTP client, `exporter_timeout` for exporter"
6. **Follows OTel patterns:** Similar to how other exporters handle dual configs
7. **Low risk:** Minimal code changes = fewer bugs
8. **Testable:** Easy to write comprehensive tests

### Implementation checklist:
```
1. Add ExporterTimeout field to Config struct
2. Add mapstructure tag: `mapstructure:"exporter_timeout"`
3. Add Unmarshal() override to handle precedence
4. Add deprecation comment (optional) noting old behavior
5. Add test cases for:
   - Only timeout set (legacy behavior)
   - Only exporter_timeout set
   - Both set (exporter_timeout wins)
6. Update README with new field documentation
7. Update testdata with examples of both timeouts
```

### Example config for users:

**Old way (still works):**
```yaml
exporters:
  prometheusremotewrite:
    endpoint: "https://cortex:9411"
    timeout: 5s  # Applies to both exporter AND HTTP client
```

**New way (independent):**
```yaml
exporters:
  prometheusremotewrite:
    endpoint: "https://cortex:9411"
    timeout: 5s          # HTTP client timeout
    exporter_timeout: 10s # Exporter timeout (different from HTTP)
```

---

## Schema Impact

**Before:**
```yaml
allOf:
  - $ref: go.opentelemetry.io/collector/exporter/exporterhelper.timeout_config
  - $ref: go.opentelemetry.io/collector/config/configretry.back_off_config
  - $ref: go.opentelemetry.io/collector/config/confighttp.client_config
# → collision on "timeout" field
```

**After (Option 1):**
```yaml
$ref: go.opentelemetry.io/collector/exporter/exporterhelper.timeout_config
properties:
  exporter_timeout:
    type: string
    format: duration
    description: "Exporter timeout. Takes precedence over timeout field."
# … rest of properties
allOf:
  - $ref: go.opentelemetry.io/collector/config/configretry.back_off_config
  - $ref: go.opentelemetry.io/collector/config/confighttp.client_config
```

**Result:** Clear, no collision, backward compatible.

---

## Summary

**Option 1 (Add ExporterTimeout)** is the clear winner:
- ✅ Solves the timeout collision problem
- ✅ Maintains backward compatibility
- ✅ Minimal code changes
- ✅ Clear user-facing API
- ✅ Low risk of bugs
- ✅ Aligns with OTel philosophy
- ✅ Easy to document and test

**Not recommended:**
- ❌ Option 2: Breaks compatibility, doesn't fully solve it
- ❌ Option 3: Doesn't enable independent configuration
- ❌ Option 4: Doesn't work (field name metadata is irrelevant when squashed)
- ❌ Option 5: Over-engineered, fragile

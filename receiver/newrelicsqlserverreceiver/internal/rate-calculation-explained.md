# Rate Calculation in OpenTelemetry Collector Components

## Overview

This document explains how rate calculation works within OpenTelemetry Collector components (receivers, processors, exporters), specifically addressing the common question: *"How can components maintain state between processing cycles when they execute every 15 seconds?"*

## Key Concepts

### 1. Component Lifecycle in OpenTelemetry Collector

OpenTelemetry Collector components (receivers, processors, exporters) are **long-lived components** that follow this lifecycle:

```
Collector Start → Component Creation → Multiple Processing Cycles → Component Shutdown
                      ↓                       ↓
                 Single Instance      Reused across cycles
```

**Important**: Components are created **once** during collector startup and **reused** across multiple processing intervals, not recreated for each cycle.

### 2. State Persistence Architecture

```go
// InstanceScraper (receiver) or similar processor is created once and persists throughout collector lifecycle
type InstanceScraper struct {
    connection    SQLConnectionInterface
    logger        *zap.Logger
    startTime     pcommon.Timestamp
    engineEdition int
    rateTracker   *rateTracker  // ← This persists between processing cycles
}

// rateTracker maintains state across processing cycles (works in receivers AND processors)
type rateTracker struct {
    previousValues map[string]float64  // Stores last known values
    previousTime   map[string]time.Time // Stores last processing timestamps
    mutex          sync.RWMutex         // Thread-safe access
}
```

## How Rate Calculation Works

### Step-by-Step Process

#### Processing Cycle 1 (T=0s)
```
Counter Value: 1000
calculateRate("metric_name", 1000, T=0s)
├── Check previousValues["metric_name"] → Not found
├── Store current values: previousValues["metric_name"] = 1000
├── Store current time: previousTime["metric_name"] = T=0s
└── Return: (0, false) → No rate available yet
```

#### Processing Cycle 2 (T=15s)
```
Counter Value: 1150
calculateRate("metric_name", 1150, T=15s)
├── Check previousValues["metric_name"] → Found: 1000
├── Check previousTime["metric_name"] → Found: T=0s
├── Calculate rate: (1150 - 1000) / (15s - 0s) = 10.0/sec
├── Store new values: previousValues["metric_name"] = 1150
├── Store new time: previousTime["metric_name"] = T=15s
└── Return: (10.0, true) → Rate calculated successfully
```

#### Processing Cycle 3 (T=30s)
```
Counter Value: 1300
calculateRate("metric_name", 1300, T=30s)
├── Check previousValues["metric_name"] → Found: 1150
├── Check previousTime["metric_name"] → Found: T=15s
├── Calculate rate: (1300 - 1150) / (30s - 15s) = 10.0/sec
├── Store new values: previousValues["metric_name"] = 1300
├── Store new time: previousTime["metric_name"] = T=30s
└── Return: (10.0, true) → Rate calculated successfully
```

### Code Implementation

```go
func (rt *rateTracker) calculateRate(metricName string, currentValue float64, currentTime time.Time) (float64, bool) {
    rt.mutex.Lock()
    defer rt.mutex.Unlock()

    // Retrieve previous values from persistent state
    previousValue, hasPrevious := rt.previousValues[metricName]
    previousTime, hasTime := rt.previousTime[metricName]

    // Update state for next calculation (CRITICAL: This persists!)
    rt.previousValues[metricName] = currentValue
    rt.previousTime[metricName] = currentTime

    // First execution: no previous data available
    if !hasPrevious || !hasTime {
        return 0, false // Skip first collection
    }

    // Calculate rate from delta
    timeDelta := currentTime.Sub(previousTime).Seconds()
    valueDelta := currentValue - previousValue
    
    // Handle edge cases
    if timeDelta <= 0 || valueDelta < 0 {
        return 0, false // Invalid or counter reset
    }

    return valueDelta / timeDelta, true
}
```

## Why This Works Across All Component Types

### 1. **Persistent Instance State**
- **Receivers**: The scraper struct lives in memory throughout the collector's lifecycle
- **Processors**: The processor instance maintains its own state tracker with persistent maps
- **Exporters**: Similar pattern can be applied for rate limiting or batching logic
- Memory is not cleared between processing cycles

### 2. **Thread-Safe Operations**
- `sync.RWMutex` ensures safe concurrent access to shared state
- Multiple metrics can be processed simultaneously without data corruption
- Works across all component types that need stateful operations

### 3. **Per-Metric State Tracking**
- Each metric is tracked independently using `metricName` as the key
- Different counters maintain separate rate calculation state
- Applies to both receivers (scraping metrics) and processors (transforming metrics)

### 4. **Graceful First Execution**
- First processing cycle gracefully handles missing previous values
- No rate is emitted until sufficient data is available (second processing cycle)
- Consistent behavior across receivers and processors

## Component-Specific Applications

### Receivers
```go
// In receivers: Calculate rates from scraped cumulative counters
if sourceType == "rate" {
    currentTime := time.Now()
    calculatedRate, hasRate := s.rateTracker.calculateRate(metricName, rawValue, currentTime)
    if hasRate {
        finalValue = calculatedRate
        shouldEmitMetric = true
    }
}
```

### Processors
```go
// In processors: Transform cumulative counters to rate metrics
func (p *rateProcessor) processMetrics(ctx context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
    // Similar rate calculation logic can be applied to transform
    // cumulative counter metrics into rate metrics during processing
    currentTime := time.Now()
    calculatedRate, hasRate := p.rateTracker.calculateRate(metricName, counterValue, currentTime)
    // ... transform metric type from counter to gauge with rate value
}
```

## Common Misconceptions

### ❌ "Components are created for each processing cycle"
**Reality**: Components (receivers, processors, exporters) are long-lived components created once during collector startup.

### ❌ "State is lost between processing cycles"
**Reality**: State persists in memory within the component instance throughout the collector's lifecycle.

### ❌ "15-second intervals reset everything"
**Reality**: The interval is just the trigger for processing; the component instance and its state remain intact.

### ❌ "This only works in receivers"
**Reality**: The same pattern works in processors, exporters, and any other collector component that needs stateful operations.

## Edge Cases Handled

### Counter Resets
```go
if valueDelta < 0 {
    // Counter reset detected (e.g., service restart)
    return 0, false // Skip this calculation
}
```

### Time Anomalies
```go
if timeDelta <= 0 {
    // Clock skew or duplicate processing
    return 0, false // Skip this calculation
}
```

### First Collection
```go
if !hasPrevious || !hasTime {
    // No previous data available
    return 0, false // Skip until next cycle
}
```

## Memory Management

### State Growth
- Each unique metric name creates one entry in the maps
- Memory usage is bounded by the number of unique metrics
- No automatic cleanup (metrics persist for component lifetime)

### Cleanup Strategy
- State is automatically cleared when the component shuts down
- Counter resets are handled gracefully without manual intervention

## Integration with OpenTelemetry Metrics

### In Receivers
```go
if sourceType == "rate" {
    currentTime := time.Now()
    calculatedRate, hasRate := s.rateTracker.calculateRate(metricName, rawValue, currentTime)
    
    if hasRate {
        // Emit calculated rate as a gauge metric
        finalValue = calculatedRate
        shouldEmitMetric = true
    } else {
        // Skip metric emission for first collection
        shouldEmitMetric = false
    }
}
```

### In Processors
```go
// Transform cumulative counters to rates during metric processing
for i := 0; i < metrics.DataPointCount(); i++ {
    dp := metrics.DataPointAt(i)
    currentValue := dp.IntValue()
    currentTime := dp.Timestamp().AsTime()
    
    rate, hasRate := p.rateTracker.calculateRate(metricKey, float64(currentValue), currentTime)
    if hasRate {
        // Convert to gauge and set rate value
        dp.SetDoubleValue(rate)
    }
}
```

## Summary

The rate calculation works across all OpenTelemetry Collector components because:

1. **Component instances persist** across processing cycles (receivers, processors, exporters)
2. **State is maintained in memory** within the component
3. **Each processing cycle reuses** the same component instance
4. **Thread-safe maps** store previous values between calls
5. **First execution is handled gracefully** by skipping rate calculation
6. **Pattern is reusable** across different component types for various stateful operations

This design leverages the OpenTelemetry Collector's architecture where all components are designed to be stateful, long-lived components rather than stateless, per-request processors.

## Component Stability Levels

When working with OpenTelemetry Collector components, it's important to understand the stability levels that indicate maturity and production-readiness:

### **Alpha** 🚧
**Meaning**: Early development stage, experimental
- **Production Use**: ❌ **NOT recommended for production**
- **API Stability**: ⚠️ Breaking changes expected without notice
- **Features**: May be incomplete or change significantly
- **Testing**: Basic functionality testing
- **Documentation**: May be incomplete
- **Support**: Limited community support

### **Beta** ✅
**Meaning**: Feature-complete and relatively stable
- **Production Use**: ✅ **Safe for production** (with caution)
- **API Stability**: ✅ Breaking changes rare, with deprecation notices
- **Features**: Core functionality complete and tested
- **Testing**: Comprehensive testing, some production usage
- **Documentation**: Complete and accurate
- **Support**: Good community support

### **Stable/GA** 🎯
**Meaning**: Production-ready, fully mature
- **Production Use**: ✅ **Fully recommended for production**
- **API Stability**: ✅ Guaranteed backward compatibility
- **Features**: Complete, well-documented, extensively tested
- **Testing**: Battle-tested in production
- **Support**: Full community and vendor support

### **Component Lifecycle**
```
Alpha → Beta → Stable/GA
  ↓       ↓       ↓
Experimental → Production-Ready → Fully Mature
```

## Alternative: Using Standard Processors

Instead of implementing rate calculation manually in receivers, consider using existing OpenTelemetry processors:

### **`cumulativetodeltaprocessor` (Beta)** ✅ **Recommended**
Converts cumulative counter metrics to delta metrics using the same stateful pattern described in this document.

**Why use it:**
- **Production-ready**: Beta stability level
- **Battle-tested**: Widely used in production
- **Standard pattern**: Follows OpenTelemetry best practices
- **Less maintenance**: No custom rate calculation code needed

**Configuration Example:**
```yaml
processors:
  cumulativetodelta:
    include:
      metrics:
        - "sqlserver.instance.*"
      match_type: regexp
    initial_value: auto  # Skip first collection
    max_staleness: 5m    # Clean up old state
```

### **`deltatorateprocessor` (Alpha)** ⚠️ **Not recommended for production**
Converts delta metrics to rate metrics (per-second rates).

**Why avoid for production:**
- **Alpha stability**: Still experimental
- **API changes**: Breaking changes without notice
- **Limited testing**: Not thoroughly tested in production

## Recommendations

1. **For Production**: Use `cumulativetodeltaprocessor` (Beta) instead of manual rate calculation
2. **For Development**: Manual implementation is fine for learning/prototyping
3. **Migration Path**: Emit cumulative sums from receiver → Use processor for rate calculation
4. **Future**: Consider migrating to processors for better separation of concerns

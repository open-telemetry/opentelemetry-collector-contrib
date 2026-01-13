# AGENT Guidelines

## Repository Overview

This is an OpenTelemetry Collector Contrib processor called **Span Pruning Processor**. The processor identifies duplicate or similar leaf spans within a single trace, groups them, and replaces each group with a single aggregated summary span.

### Key Features
- Groups similar leaf spans by name, status code, attributes, and parent span name
- Supports glob patterns for attribute matching (e.g., `db.*`, `http.*`)
- Recursive parent aggregation when all children are aggregated
- Configurable aggregation thresholds and depth limits
- Histogram support for latency distribution tracking

### Project Structure
```
spanpruningprocessor/
├── config.go              # Configuration struct validation
├── factory.go             # Factory implementation with NewFactory()
├── processor.go           # Main processing logic
├── tree.go                # Tree-based trace analysis utilities
├── grouping.go            # Span grouping algorithms  
├── aggregation.go         # Aggregation logic and statistics
├── stats.go               # Statistics collection helpers
├── metadata.yaml          # Component metadata (generates code)
├── doc.go                 # Package documentation with mdatagen directive
├── *.go                   # Various test files
└── internal/
    └── metadata/          # Generated telemetry code
```

## Essential Commands

### Build & Test
```bash
# Run all tests (unit and integration)
go test ./...

# Run specific test file
go test -run TestLeafSpanPruning_BasicAggregation ./...

# Run benchmarks
go test -bench=. ./...

# Run with verbose output for debugging
go test -v ./...
```

### Code Generation
```bash
# Generate code from metadata.yaml (required after changes)
go generate ./...

# Format Go code
gofmt -w .

# Check formatting issues
gofmt -l .
```

### Static Analysis & Quality
```bash
# Run go vet for basic static analysis
go vet ./...

# Check for unused imports (if golint is available)
golint ./...
```

## Code Organization and Patterns

### Package Structure
- **Main package**: `spanpruningprocessor`
- **Configuration**: `Config` struct in config.go with validation
- **Processor implementation**: `spanPruningProcessor` type in processor.go
- **Factory pattern**: Standard OTEL collector factory in factory.go

### Key Interfaces and Types
```go
// Configuration must implement component.Config interface
type Config struct {
    GroupByAttributes           []string          `mapstructure:"group_by_attributes"`
    MinSpansToAggregate        int               `mapstructure:"min_spans_to_aggregate"`
    MaxParentDepth             int               `mapstructure:"max_parent_depth"`
    AggregationSpanNameSuffix  string            `mapstructure:"aggregation_span_name_suffix"`
    AggregationAttributePrefix string            `mapstructure:"aggregation_attribute_prefix"`
    AggregationHistogramBuckets []time.Duration   `mapstructure:"aggregation_histogram_buckets"`
}

// Main processor type
type spanPruningProcessor struct {
    config            *Config
    logger            *zap.Logger
    attributePatterns []attributePattern  // Compiled glob patterns
    telemetryBuilder  *metadata.TelemetryBuilder
}
```

### OTEL Collector Integration Patterns
- Uses `processorhelper.NewTraces()` for standard processor setup
- Implements `consumer.Traces` interface with `processTraces(ctx, td ptrace.Traces) (ptrace.Traces, error)`
- Telemetry integration via `metadata.TelemetryBuilder`
- Configuration validation through `Validate() error` method

### Trace Processing Pipeline
1. **Group spans by TraceID** - Organize incoming trace data
2. **Build trace tree** - Create hierarchical span relationships  
3. **Identify leaf nodes** - Find spans with no children
4. **Group similar leaves** - Group by name, status, attributes, parent name
5. **Analyze parent aggregations** - Recursively find eligible parents
6. **Execute aggregations** - Replace groups with summary spans

## Testing Approach and Patterns

### Test Structure
- Uses `testify` framework for assertions (`assert`, `require`)
- Comprehensive test coverage in `*_test.go` files
- Helper functions create consistent test trace data
- Benchmark tests for performance validation in `*_benchmark_test.go`

### Common Test Helpers
```go
// Create test traces with specific patterns
createTestTraceWithLeafSpans(t, numLeafSpans int, spanName string, attrs map[string]string)
createTestTraceWithMixedOperations(t *testing.T) 
countSpans(td ptrace.Traces) int
findSpanByNameSuffix(td ptrace.Traces, suffix string) ptrace.Span

// Factory setup pattern
factory := NewFactory()
cfg := factory.CreateDefaultConfig().(*Config)
tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
```

### Test Categories
1. **Basic aggregation tests** - Verify core functionality with simple traces
2. **Configuration validation tests** - Ensure proper config handling  
3. **Glob pattern tests** - Validate attribute matching patterns
4. **Edge case tests** - Orphan spans, multiple roots, missing parents
5. **Recursive aggregation tests** - Multi-level parent aggregation
6. **Performance benchmarks** - Measure processing efficiency

## Important Conventions and Gotchas

### Configuration Validation
- `MinSpansToAggregate` must be ≥ 2 (validation enforces this)
- Glob patterns in `GroupByAttributes` are validated at creation time  
- Empty suffix/prefix values will cause validation errors
- Histogram buckets must be positive and ascending order

### Trace Processing Details
- **Leaf spans** are identified as spans not referenced by any other span
- Parent aggregation only occurs when ALL children are aggregated
- Root spans (no parent) are never aggregated, even if all children are
- Summary spans inherit attributes from the first span in each group

### Performance Considerations
- Tree-based processing for efficiency with large traces
- Glob patterns compiled once at processor creation  
- Mark-and-sweep approach to avoid duplicate work
- Two-phase processing (analysis then execution) for correctness

### Common Pitfalls
1. **Glob pattern syntax** - Use valid glob patterns, not regex
2. **Threshold configuration** - Remember `min_spans_to_aggregate` applies per group
3. **Parent-child relationships** - Aggregation preserves original parent span IDs
4. **Status code grouping** - Spans with different statuses are always separate groups

## Development Workflow

### Making Changes
1. Update `Config` struct and validation in config.go
2. Modify processor logic in processor.go, tree.go, grouping.go, etc.
3. Add tests for new functionality
4. Run `go generate ./` if metadata.yaml changes
5. Run full test suite: `go test -v ./...`
6. Format code: `gofmt -w .`

### Adding New Configuration Options
1. Add field to `Config` struct with appropriate tags
2. Update validation logic in `Validate()` method  
3. Set default value in `createDefaultConfig()`
4. Update documentation and examples
5. Add tests for new configuration edge cases

### Testing Guidelines
- Every new feature should have corresponding tests
- Test both success and failure paths
- Include edge cases (empty traces, single spans, etc.)
- Use descriptive test names that explain the scenario
- Leverage existing helper functions where possible
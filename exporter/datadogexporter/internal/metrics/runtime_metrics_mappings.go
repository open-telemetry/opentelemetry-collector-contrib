package metrics

// goRuntimeMetricsMappings defines the mappings from OTel runtime metric names to their
// equivalent Datadog runtime metric names
var goRuntimeMetricsMappings = map[string]string{
	"process.runtime.go.goroutines":        "runtime.go.num_goroutine",
	"process.runtime.go.cgo.calls":         "runtime.go.num_cgo_call",
	"process.runtime.go.lookups":           "runtime.go.mem_stats.lookups",
	"process.runtime.go.mem.heap_alloc":    "runtime.go.mem_stats.heap_alloc",
	"process.runtime.go.mem.heap_sys":      "runtime.go.mem_stats.heap_sys",
	"process.runtime.go.mem.heap_idle":     "runtime.go.mem_stats.heap_idle",
	"process.runtime.go.mem.heap_inuse":    "runtime.go.mem_stats.heap_inuse",
	"process.runtime.go.mem.heap_released": "runtime.go.mem_stats.heap_released",
	"process.runtime.go.mem.heap_objects":  "runtime.go.mem_stats.heap_objects",
	"process.runtime.go.gc.pause_total_ns": "runtime.go.mem_stats.pause_total_ns",
	"process.runtime.go.gc.count":          "runtime.go.mem_stats.num_gc",
}

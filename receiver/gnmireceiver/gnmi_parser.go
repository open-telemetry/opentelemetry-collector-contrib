// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnmireceiver

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gnmi "github.com/openconfig/gnmi/proto/gnmi"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gnmireceiver/internal"
)

// gnmiParser transforms gNMI SubscribeResponse messages into pmetric.Metrics.
//
// Counter vs Gauge resolution uses two levels, applied in order:
//
//	Level 1 — YANG file lookup (most precise)
//	          The YANGParser loaded at startup resolves the exact type from
//	          the .yang files configured in module_paths. If it returns a
//	          non-nil YANGDataType, that result wins unconditionally.
//
//	Level 2 — Structural node analysis (semantic fallback)
//	          Inspects the gNMI path elements directly:
//	            - A path element named "counters" or "statistics" means every
//	              leaf under it is a cumulative counter by OpenConfig convention.
//	            - A path element named "state" alone is ambiguous — we look at
//	              the leaf name to disambiguate (rate/util/percent → Gauge).
//	          No arbitrary word lists: the decision is structural, not lexical.
//
// If both levels are inconclusive the metric is emitted as a Gauge (safe default).
type gnmiParser struct {
	logger     *zap.Logger
	yangParser *internal.YANGParser // nil when no module_paths are configured
}

// newGNMIParser creates a new parser.
// yangParser may be nil — in that case only level 2 is used.
func newGNMIParser(logger *zap.Logger, yangParser *internal.YANGParser) *gnmiParser {
	return &gnmiParser{
		logger:     logger,
		yangParser: yangParser,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Public API
// ─────────────────────────────────────────────────────────────────────────────

// parse converts a gNMI SubscribeResponse into OTel Metrics.
func (p *gnmiParser) parse(resp *gnmi.SubscribeResponse, target string) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()

	notification := resp.GetUpdate()
	if notification == nil {
		// SyncResponse — nothing to emit
		return metrics, nil
	}

	rm := metrics.ResourceMetrics().AppendEmpty()
	resAttrs := rm.Resource().Attributes()
	resAttrs.PutStr("device.address", target)
	resAttrs.PutStr("host", target)

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("otelcol/gnmireceiver")

	ts := pcommon.Timestamp(notification.Timestamp)
	if notification.Timestamp == 0 {
		ts = pcommon.NewTimestampFromTime(time.Now())
	}

	for _, update := range notification.Update {
		p.handleUpdate(sm.Metrics(), update, ts)
	}

	return metrics, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Update processing
// ─────────────────────────────────────────────────────────────────────────────

func (p *gnmiParser) handleUpdate(mCol pmetric.MetricSlice, update *gnmi.Update, ts pcommon.Timestamp) {
	if update.Path == nil || update.Val == nil {
		return
	}

	metricName, attributes := p.extractMetadata(update.Path)

	val := update.Val
	switch v := val.GetValue().(type) {

	case *gnmi.TypedValue_UintVal:
		p.emitNumeric(mCol, metricName, update.Path, float64(v.UintVal), ts, attributes)

	case *gnmi.TypedValue_IntVal:
		p.emitNumeric(mCol, metricName, update.Path, float64(v.IntVal), ts, attributes)

	case *gnmi.TypedValue_FloatVal:
		// Floats are always instantaneous (rates, ratios) — always Gauge
		p.addGaugePoint(mCol, metricName, float64(v.FloatVal), ts, attributes)

	case *gnmi.TypedValue_BoolVal:
		boolFloat := 0.0
		if v.BoolVal {
			boolFloat = 1.0
		}
		p.addGaugePoint(mCol, metricName, boolFloat, ts, attributes)

	case *gnmi.TypedValue_JsonVal:
		p.handleJSONUpdate(mCol, metricName, update.Path, v.JsonVal, ts, attributes)

	case *gnmi.TypedValue_JsonIetfVal:
		p.handleJSONUpdate(mCol, metricName, update.Path, v.JsonIetfVal, ts, attributes)

	case *gnmi.TypedValue_StringVal:
		// String state values → info metric (value stored as attribute)
		p.addInfoPoint(mCol, metricName, v.StringVal, ts, attributes)

	default:
		p.logger.Debug("Unsupported gNMI TypedValue type, skipping",
			zap.String("metric", metricName))
	}
}

// emitNumeric resolves Counter vs Gauge for a numeric value and emits the metric.
func (p *gnmiParser) emitNumeric(
	mCol pmetric.MetricSlice,
	metricName string,
	path *gnmi.Path,
	val float64,
	ts pcommon.Timestamp,
	attrs map[string]string,
) {
	if p.resolveIsCounter(path, metricName) {
		p.addSumPoint(mCol, metricName, val, ts, attrs)
	} else {
		p.addGaugePoint(mCol, metricName, val, ts, attrs)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Counter vs Gauge resolution — 2 levels
// ─────────────────────────────────────────────────────────────────────────────

// resolveIsCounter applies the two-level resolution strategy.
// Returns true → emit as Sum (counter), false → emit as Gauge.
func (p *gnmiParser) resolveIsCounter(path *gnmi.Path, metricName string) bool {
	// ── Level 1: YANG file lookup ─────────────────────────────────────────────
	// If the YANGParser is configured and knows this path, its answer is final.
	if p.yangParser != nil {
		encodingPath := buildEncodingPath(path)
		leafName := lastElem(path)

		if yType := p.yangParser.GetDataTypeForEncodingPath(encodingPath, leafName); yType != nil {
			// YANG file gave us a definitive answer — use it unconditionally
			return yType.IsCounterType()
		}
	}

	// ── Level 2: Structural node analysis ────────────────────────────────────
	return p.structuralIsCounter(path, metricName)
}

// structuralIsCounter analyses the gNMI path elements.
//
// Precision ranking (highest to lowest):
//
//	P3 — "counters" or "statistics" node present
//	     → ALL leaves under it are counters by OpenConfig definition.
//	     Overrides everything below.
//
//	P2 — "state" node present without "counters"/"statistics"
//	     → ambiguous container; examine the leaf name for rate/util patterns.
//	     If the leaf looks like a rate → Gauge. Otherwise → Counter.
//
//	P1 — "config" node present
//	     → configuration leaves, never metrics → Gauge.
//
//	P0 — no structural node recognised → Gauge (safe default).
func (p *gnmiParser) structuralIsCounter(path *gnmi.Path, metricName string) bool {
	if path == nil {
		return false
	}

	elemNames := make([]string, 0, len(path.Elem))
	for _, e := range path.Elem {
		elemNames = append(elemNames, strings.ToLower(e.Name))
	}

	hasCounters := containsElem(elemNames, "counters")
	hasStatistics := containsElem(elemNames, "statistics")
	hasState := containsElem(elemNames, "state")
	hasConfig := containsElem(elemNames, "config")

	// P3 — explicit counter container
	if hasCounters || hasStatistics {
		return true
	}

	// P1 — config container → never a counter
	if hasConfig {
		return false
	}

	// P2 — "state" without counters/statistics — look at the leaf name
	if hasState {
		leaf := strings.ToLower(lastElem(path))
		for _, ratePattern := range gaugeLeafPatterns {
			if strings.Contains(leaf, ratePattern) {
				return false
			}
		}
		// No rate pattern matched → lean Counter for numeric state leaves
		return true
	}

	// P0 — no structural hint → Gauge
	return false
}

// gaugeLeafPatterns are leaf-name substrings that indicate an instantaneous
// measurement even when found under a "state" container.
// Kept deliberately short: only unambiguous rate/util indicators.
var gaugeLeafPatterns = []string{
	"rate",
	"utilization",
	"percent",
	"kbps", "mbps", "gbps", "bps",
	"pps",
	"temperature",
	"voltage",
	"frequency",
	"load",
	"latency",
	"instant", // openconfig-platform: input-power/instant
	"avg", "min", "max",
}

// ─────────────────────────────────────────────────────────────────────────────
// Path helpers
// ─────────────────────────────────────────────────────────────────────────────

// extractMetadata parses a gNMI Path into a dotted metric name and a map of
// path-element keys (e.g. interface[name=Eth1] → attrs["name"]="Eth1").
func (p *gnmiParser) extractMetadata(path *gnmi.Path) (string, map[string]string) {
	attrs := make(map[string]string)
	var parts []string

	if path.Origin != "" {
		parts = append(parts, path.Origin)
	}

	for _, elem := range path.Elem {
		if elem.Name != "" {
			parts = append(parts, elem.Name)
		}
		for k, v := range elem.Key {
			attrs[k] = v
		}
	}

	return strings.Join(parts, "."), attrs
}

// buildEncodingPath builds a string "origin:elem1/elem2/…"
// compatible with YANGParser.GetDataTypeForEncodingPath.
func buildEncodingPath(path *gnmi.Path) string {
	if path == nil {
		return ""
	}
	var elems []string
	for _, e := range path.Elem {
		if e.Name != "" {
			elems = append(elems, e.Name)
		}
	}
	body := strings.Join(elems, "/")
	if path.Origin != "" {
		return path.Origin + ":" + body
	}
	return body
}

// lastElem returns the name of the last path element (the leaf name).
func lastElem(path *gnmi.Path) string {
	if path == nil || len(path.Elem) == 0 {
		return ""
	}
	return path.Elem[len(path.Elem)-1].Name
}

// containsElem reports whether name appears in the slice.
func containsElem(elems []string, name string) bool {
	for _, e := range elems {
		if e == name {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON handling
// ─────────────────────────────────────────────────────────────────────────────

func (p *gnmiParser) handleJSONUpdate(
	mCol pmetric.MetricSlice,
	baseName string,
	path *gnmi.Path,
	jsonRaw []byte,
	ts pcommon.Timestamp,
	attrs map[string]string,
) {
	var data interface{}
	if err := json.Unmarshal(jsonRaw, &data); err != nil {
		p.logger.Error("Failed to unmarshal gNMI JSON payload",
			zap.String("metric", baseName),
			zap.Error(err))
		return
	}
	p.flattenJSON(mCol, baseName, path, data, ts, attrs)
}

// flattenJSON recursively walks a JSON value and emits a metric for each leaf.
// The original gNMI path is carried so the structural resolver stays accurate.
func (p *gnmiParser) flattenJSON(
	mCol pmetric.MetricSlice,
	baseName string,
	path *gnmi.Path,
	data interface{},
	ts pcommon.Timestamp,
	attrs map[string]string,
) {
	switch v := data.(type) {

	case map[string]interface{}:
		for key, val := range v {
			childPath := appendPathElem(path, key)
			p.flattenJSON(mCol, baseName+"."+key, childPath, val, ts, attrs)
		}

	case []interface{}:
		for i, item := range v {
			childName := fmt.Sprintf("%s.%d", baseName, i)
			p.flattenJSON(mCol, childName, path, item, ts, attrs)
		}

	case float64:
		p.emitNumeric(mCol, baseName, path, v, ts, attrs)

	case bool:
		val := 0.0
		if v {
			val = 1.0
		}
		p.addGaugePoint(mCol, baseName, val, ts, attrs)

	case string:
		p.addInfoPoint(mCol, baseName, v, ts, attrs)
	}
}

// appendPathElem returns a new gnmi.Path with an extra element appended.
func appendPathElem(base *gnmi.Path, name string) *gnmi.Path {
	origin := ""
	newElems := make([]*gnmi.PathElem, 0)
	if base != nil {
		origin = base.Origin
		newElems = append(newElems, base.Elem...)
	}
	newElems = append(newElems, &gnmi.PathElem{Name: name})
	return &gnmi.Path{Origin: origin, Elem: newElems}
}

// ─────────────────────────────────────────────────────────────────────────────
// OTel metric helpers
// ─────────────────────────────────────────────────────────────────────────────

func (p *gnmiParser) addSumPoint(
	mCol pmetric.MetricSlice,
	name string, val float64,
	ts pcommon.Timestamp,
	attrs map[string]string,
) {
	m := mCol.AppendEmpty()
	m.SetName(name)
	sum := m.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)
	dp.SetDoubleValue(val)
	applyAttrs(dp.Attributes(), attrs)
}

func (p *gnmiParser) addGaugePoint(
	mCol pmetric.MetricSlice,
	name string, val float64,
	ts pcommon.Timestamp,
	attrs map[string]string,
) {
	m := mCol.AppendEmpty()
	m.SetName(name)
	gauge := m.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)
	dp.SetDoubleValue(val)
	applyAttrs(dp.Attributes(), attrs)
}

func (p *gnmiParser) addInfoPoint(
	mCol pmetric.MetricSlice,
	name, strVal string,
	ts pcommon.Timestamp,
	attrs map[string]string,
) {
	m := mCol.AppendEmpty()
	m.SetName(name + "_info")
	gauge := m.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(ts)
	dp.SetDoubleValue(1.0)
	dp.Attributes().PutStr("value", strVal)
	applyAttrs(dp.Attributes(), attrs)
}

func applyAttrs(dest pcommon.Map, attrs map[string]string) {
	for k, v := range attrs {
		if v != "" {
			dest.PutStr(k, v)
		}
	}
}

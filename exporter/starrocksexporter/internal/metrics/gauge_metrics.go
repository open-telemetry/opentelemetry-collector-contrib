// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal/metrics"

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/starrocksexporter/internal"
)

type gaugeModel struct {
	metricName        string
	metricDescription string
	metricUnit        string
	metadata          *MetricsMetaData
	gauge             pmetric.Gauge
}

type gaugeMetrics struct {
	gaugeModels []*gaugeModel
	insertSQL   string
	count       int
}

func (g *gaugeMetrics) insert(ctx context.Context, db *sql.DB) error {
	if g.count == 0 {
		return nil
	}

	processStart := time.Now()

	stmt, err := db.PrepareContext(ctx, g.insertSQL)
	if err != nil {
		return fmt.Errorf("failed to prepare gauge insert statement: %w", err)
	}
	defer stmt.Close()

	for _, model := range g.gaugeModels {
		resAttrJSON, _ := internal.AttributesToJSON(model.metadata.ResAttr)
		scopeAttrJSON, _ := internal.AttributesToJSON(model.metadata.ScopeInstr.Attributes())
		serviceName := internal.GetServiceName(model.metadata.ResAttr)

		for i := 0; i < model.gauge.DataPoints().Len(); i++ {
			dp := model.gauge.DataPoints().At(i)
			attrsJSON, _ := internal.AttributesToJSON(dp.Attributes())
			exemplarsJSON, _ := convertExemplarsJSON(dp.Exemplars())

			_, execErr := stmt.ExecContext(ctx,
				serviceName,
				model.metricName,
				dp.Timestamp().AsTime(),
				resAttrJSON,
				model.metadata.ResURL,
				model.metadata.ScopeInstr.Name(),
				model.metadata.ScopeInstr.Version(),
				scopeAttrJSON,
				model.metadata.ScopeInstr.DroppedAttributesCount(),
				model.metadata.ScopeURL,
				model.metricDescription,
				model.metricUnit,
				attrsJSON,
				dp.StartTimestamp().AsTime(),
				getValue(dp.IntValue(), dp.DoubleValue(), dp.ValueType()),
				uint32(dp.Flags()),
				exemplarsJSON,
			)
			if execErr != nil {
				return fmt.Errorf("failed to execute gauge metric insert: %w", execErr)
			}
		}
	}

	processDuration := time.Since(processStart)
	logger.Debug("insert gauge metrics",
		zap.Int("records", g.count),
		zap.String("process_cost", processDuration.String()))

	return nil
}

func (g *gaugeMetrics) Add(resAttr pcommon.Map, resURL string, scopeInstr pcommon.InstrumentationScope, scopeURL string, metrics pmetric.Metric) {
	gauge := metrics.Gauge()
	g.count += gauge.DataPoints().Len()
	g.gaugeModels = append(g.gaugeModels, &gaugeModel{
		metricName:        metrics.Name(),
		metricDescription: metrics.Description(),
		metricUnit:        metrics.Unit(),
		metadata: &MetricsMetaData{
			ResAttr:    resAttr,
			ResURL:     resURL,
			ScopeURL:   scopeURL,
			ScopeInstr: scopeInstr,
		},
		gauge: gauge,
	})
}

func getValue(intVal int64, doubleVal float64, valueType any) float64 {
	switch t := valueType.(type) {
	case pmetric.ExemplarValueType:
		switch t {
		case pmetric.ExemplarValueTypeDouble:
			return doubleVal
		case pmetric.ExemplarValueTypeInt:
			return float64(intVal)
		case pmetric.ExemplarValueTypeEmpty:
			return 0.0
		default:
			return 0.0
		}
	case pmetric.NumberDataPointValueType:
		switch t {
		case pmetric.NumberDataPointValueTypeDouble:
			return doubleVal
		case pmetric.NumberDataPointValueTypeInt:
			return float64(intVal)
		case pmetric.NumberDataPointValueTypeEmpty:
			return 0.0
		default:
			return 0.0
		}
	default:
		return 0.0
	}
}

func convertExemplarsJSON(exemplars pmetric.ExemplarSlice) (string, error) {
	exemplarList := make([]map[string]interface{}, 0, exemplars.Len())
	for i := 0; i < exemplars.Len(); i++ {
		exemplar := exemplars.At(i)
		exemplarMap := map[string]interface{}{
			"time_unix": exemplar.Timestamp().AsTime(),
			"value":     getValue(exemplar.IntValue(), exemplar.DoubleValue(), exemplar.ValueType()),
		}
		if exemplar.TraceID().IsEmpty() {
			exemplarMap["trace_id"] = ""
		} else {
			exemplarMap["trace_id"] = exemplar.TraceID().String()
		}
		if exemplar.SpanID().IsEmpty() {
			exemplarMap["span_id"] = ""
		} else {
			exemplarMap["span_id"] = exemplar.SpanID().String()
		}
		filteredAttrs, _ := internal.AttributesToJSON(exemplar.FilteredAttributes())
		var attrsMap map[string]interface{}
		if json.Unmarshal([]byte(filteredAttrs), &attrsMap) == nil {
			exemplarMap["filtered_attributes"] = attrsMap
		}
		exemplarList = append(exemplarList, exemplarMap)
	}
	jsonBytes, err := json.Marshal(exemplarList)
	if err != nil {
		return "[]", err
	}
	return string(jsonBytes), nil
}

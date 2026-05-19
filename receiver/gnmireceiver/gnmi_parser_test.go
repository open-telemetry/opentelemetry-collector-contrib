package gnmireceiver

import (
	"testing"

	gnmi "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestParser_StructuralResolution(t *testing.T) {
	parser := newGNMIParser(zap.NewNop(), nil) // Sans YANG pour tester le Level 2

	tests := []struct {
		name       string
		path       *gnmi.Path
		val        float64
		expectType pmetric.MetricType
	}{
		{
			name:       "Counter via 'counters' node",
			path:       &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}, {Name: "interface"}, {Name: "state"}, {Name: "counters"}, {Name: "in-octets"}}},
			val:        1000,
			expectType: pmetric.MetricTypeSum,
		},
		{
			name:       "Gauge via rate pattern",
			path:       &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}, {Name: "interface"}, {Name: "state"}, {Name: "in-utilization"}}},
			val:        45.5,
			expectType: pmetric.MetricTypeGauge,
		},
		{
			name:       "Default to Gauge for config",
			path:       &gnmi.Path{Elem: []*gnmi.PathElem{{Name: "interfaces"}, {Name: "interface"}, {Name: "config"}, {Name: "enabled"}}},
			val:        1,
			expectType: pmetric.MetricTypeGauge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := pmetric.NewMetricSlice()
			tt.path.Origin = "openconfig"
			parser.emitNumeric(metrics, "test_metric", tt.path, tt.val, 0, nil)

			assert.Equal(t, 1, metrics.Len())
			assert.Equal(t, tt.expectType, metrics.At(0).Type())
		})
	}
}

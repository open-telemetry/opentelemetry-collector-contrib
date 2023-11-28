package identity

import (
	"encoding/hex"
	"hash/fnv"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	_ map[Metric]struct{}
	_ map[Series]struct{}
)

type Metric struct {
	fnv64a uint64
}

func OfMetric(res pcommon.Resource, scope pcommon.InstrumentationScope, metric pmetric.Metric) Metric {
	sum := fingerprint(
		res.Attributes(),

		scope.Name(),
		scope.Version(),
		scope.Attributes(),

		metric.Name(),
		metric.Unit(),
		metric.Type().String(),
	)
	return Metric{fnv64a: sum}
}

func fingerprint(vs ...any) uint64 {
	const Separator byte = 255
	sum := fnv.New64a()

	for _, v := range vs {
		switch t := v.(type) {
		case pcommon.Map:
			h := pdatautil.MapHash(t)
			sum.Write(h[:])
		case string:
			sum.Write([]byte(t))
		}
		sum.Write([]byte{Separator})
	}

	return sum.Sum64()
}

func (id Metric) String() string {
	return strconv.FormatUint(id.fnv64a, 16)
}

type Series struct {
	hash [16]byte
}

func OfSeries(attrs pcommon.Map) Series {
	return Series{hash: pdatautil.MapHash(attrs)}
}

func (s Series) String() string {
	return hex.EncodeToString(s.hash[:])
}
